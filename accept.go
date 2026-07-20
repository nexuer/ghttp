package ghttp

import (
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/nexuer/ghttp/encoding"
)

type codecOffer struct {
	contentType string
	typeName    string
	subtype     string
	codec       encoding.Codec
}

// AcceptNegotiator selects a codec from the representations a server supports.
// It is immutable after construction and safe for concurrent use.
type AcceptNegotiator struct {
	offers []codecOffer
}

// NewAcceptNegotiator creates a negotiator from concrete content types supported
// by a server. Offer order is the server preference when client weights are equal.
func NewAcceptNegotiator(contentTypes ...string) (*AcceptNegotiator, error) {
	if len(contentTypes) == 0 {
		return nil, fmt.Errorf("accept negotiator: no content types")
	}

	n := &AcceptNegotiator{offers: make([]codecOffer, 0, len(contentTypes))}
	for _, contentType := range contentTypes {
		contentType = strings.TrimSpace(contentType)
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return nil, fmt.Errorf("accept negotiator: invalid content type %q: %w", contentType, err)
		}
		typeName, subtype, ok := splitMediaType(mediaType)
		if !ok || typeName == "*" || subtype == "*" {
			return nil, fmt.Errorf("accept negotiator: content type %q must be concrete", contentType)
		}

		codec := CodecForContentType(contentType)
		if codec == nil {
			return nil, fmt.Errorf("accept negotiator: no codec for content type %q", contentType)
		}
		n.offers = append(n.offers, codecOffer{
			contentType: contentType,
			typeName:    typeName,
			subtype:     subtype,
			codec:       codec,
		})
	}
	return n, nil
}

// Negotiate selects a supported codec using the request's Accept header.
// The returned content type is the selected server offer. A missing or empty
// Accept header accepts the first offer. No match returns a nil codec and false.
func (n *AcceptNegotiator) Negotiate(r *http.Request) (encoding.Codec, string, bool) {
	if r == nil {
		return nil, "", false
	}
	return n.NegotiateAccept(r.Header.Values("Accept")...)
}

// NegotiateAccept selects a supported codec from one or more Accept field values.
// Media-type parameters other than q do not affect codec selection.
func (n *AcceptNegotiator) NegotiateAccept(values ...string) (encoding.Codec, string, bool) {
	if n == nil || len(n.offers) == 0 {
		return nil, "", false
	}

	nonEmpty := 0
	var single string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			nonEmpty++
			single = value
		}
	}
	if nonEmpty == 0 {
		return n.result(0)
	}
	if nonEmpty == 1 && strings.IndexAny(single, ",;\"") == -1 {
		return n.negotiateSimple(single)
	}

	type offerScore struct {
		quality     int
		specificity int
		matched     bool
	}
	var scoreBuffer [8]offerScore
	var scores []offerScore
	if len(n.offers) <= len(scoreBuffer) {
		scores = scoreBuffer[:len(n.offers)]
	} else {
		scores = make([]offerScore, len(n.offers))
	}

	validRange := false
	for _, value := range values {
		for start := 0; start < len(value); {
			mediaRange, next := nextMediaRange(value, start)
			start = next
			mediaRange = strings.TrimSpace(mediaRange)
			if mediaRange == "" {
				continue
			}

			typeName, subtype, quality, ok := parseAcceptMediaRange(mediaRange)
			if !ok {
				continue
			}
			validRange = true

			for i := range n.offers {
				specificity := matchSpecificity(typeName, subtype, n.offers[i])
				if specificity < 0 {
					continue
				}
				score := &scores[i]
				if !score.matched || specificity > score.specificity ||
					(specificity == score.specificity && quality > score.quality) {
					score.quality = quality
					score.specificity = specificity
					score.matched = true
				}
			}
		}
	}
	if !validRange {
		return nil, "", false
	}

	best := -1
	for i, score := range scores {
		if !score.matched || score.quality == 0 {
			continue
		}
		if best == -1 || score.quality > scores[best].quality {
			best = i
		}
	}
	if best == -1 {
		return nil, "", false
	}
	return n.result(best)
}

func (n *AcceptNegotiator) negotiateSimple(mediaRange string) (encoding.Codec, string, bool) {
	typeName, subtype, ok := splitMediaType(mediaRange)
	if !ok || (typeName == "*" && subtype != "*") {
		return nil, "", false
	}
	for i, offer := range n.offers {
		if matchSpecificity(typeName, subtype, offer) >= 0 {
			return n.result(i)
		}
	}
	return nil, "", false
}

func (n *AcceptNegotiator) result(index int) (encoding.Codec, string, bool) {
	offer := n.offers[index]
	return offer.codec, offer.contentType, true
}

func splitMediaType(mediaType string) (string, string, bool) {
	slash := strings.IndexByte(mediaType, '/')
	if slash <= 0 || slash == len(mediaType)-1 || strings.IndexByte(mediaType[slash+1:], '/') >= 0 {
		return "", "", false
	}
	return mediaType[:slash], mediaType[slash+1:], true
}

func matchSpecificity(typeName, subtype string, offer codecOffer) int {
	switch {
	case typeName == "*" && subtype == "*":
		return 0
	case !strings.EqualFold(typeName, offer.typeName):
		return -1
	case subtype == "*":
		return 1
	case strings.EqualFold(subtype, offer.subtype):
		return 2
	default:
		return -1
	}
}

func nextMediaRange(value string, start int) (string, int) {
	quoted := false
	escaped := false
	for i := start; i < len(value); i++ {
		switch value[i] {
		case '\\':
			if quoted {
				escaped = !escaped
			}
		case '"':
			if !escaped {
				quoted = !quoted
			}
			escaped = false
		case ',':
			if !quoted {
				return value[start:i], i + 1
			}
			escaped = false
		default:
			escaped = false
		}
	}
	return value[start:], len(value)
}

func parseAcceptMediaRange(value string) (string, string, int, bool) {
	parameterStart := strings.IndexByte(value, ';')
	mediaType := value
	if parameterStart >= 0 {
		mediaType = value[:parameterStart]
	}
	mediaType = strings.TrimSpace(mediaType)
	typeName, subtype, ok := splitMediaType(mediaType)
	if !ok || !validToken(typeName) || !validToken(subtype) ||
		(typeName == "*" && subtype != "*") {
		return "", "", 0, false
	}

	quality := 1000
	qualitySeen := false
	for parameterStart >= 0 && parameterStart < len(value) {
		i := parameterStart + 1
		for i < len(value) && isOWS(value[i]) {
			i++
		}
		nameStart := i
		for i < len(value) && isTokenChar(value[i]) {
			i++
		}
		if i == nameStart {
			return "", "", 0, false
		}
		name := value[nameStart:i]
		for i < len(value) && isOWS(value[i]) {
			i++
		}
		if i >= len(value) || value[i] != '=' {
			return "", "", 0, false
		}
		i++
		for i < len(value) && isOWS(value[i]) {
			i++
		}
		if i >= len(value) {
			return "", "", 0, false
		}

		quoted := value[i] == '"'
		valueStart := i
		if quoted {
			i++
			closed := false
			for i < len(value) {
				switch value[i] {
				case '\\':
					if i+1 >= len(value) ||
						(value[i+1] != '\t' && (value[i+1] < ' ' || value[i+1] == 0x7f)) {
						return "", "", 0, false
					}
					i += 2
				case '"':
					i++
					closed = true
				default:
					if value[i] != '\t' && (value[i] < ' ' || value[i] == 0x7f) {
						return "", "", 0, false
					}
					i++
				}
				if closed {
					break
				}
			}
			if !closed {
				return "", "", 0, false
			}
		} else {
			for i < len(value) && isTokenChar(value[i]) {
				i++
			}
			if i == valueStart {
				return "", "", 0, false
			}
		}
		parameterValue := value[valueStart:i]

		if strings.EqualFold(name, "q") {
			if qualitySeen || quoted {
				return "", "", 0, false
			}
			quality, ok = acceptQuality(parameterValue)
			if !ok {
				return "", "", 0, false
			}
			qualitySeen = true
		}

		for i < len(value) && isOWS(value[i]) {
			i++
		}
		if i == len(value) {
			parameterStart = -1
		} else if value[i] == ';' {
			parameterStart = i
		} else {
			return "", "", 0, false
		}
	}
	return typeName, subtype, quality, true
}

func validToken(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !isTokenChar(value[i]) {
			return false
		}
	}
	return true
}

func isTokenChar(c byte) bool {
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

func isOWS(c byte) bool {
	return c == ' ' || c == '\t'
}

func acceptQuality(value string) (int, bool) {
	if value == "" {
		return 1000, true
	}
	if value == "0" {
		return 0, true
	}
	if value == "1" {
		return 1000, true
	}
	if len(value) < 2 || value[1] != '.' || len(value) > 5 {
		return 0, false
	}

	fraction := value[2:]
	quality := 0
	for i := 0; i < len(fraction); i++ {
		if fraction[i] < '0' || fraction[i] > '9' {
			return 0, false
		}
		quality = quality*10 + int(fraction[i]-'0')
	}
	for i := len(fraction); i < 3; i++ {
		quality *= 10
	}

	switch value[0] {
	case '0':
		return quality, true
	case '1':
		return 1000, quality == 0
	default:
		return 0, false
	}
}
