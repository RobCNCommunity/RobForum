package store

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var (
	hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	domainPattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	hostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`)
)

func validateWebURL(value string, allowRelative bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if containsControl(value) || len(value) > 500 {
		return errors.New("URL is invalid")
	}
	if allowRelative && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		parsed, err := url.ParseRequestURI(value)
		if err == nil && parsed.Host == "" {
			return nil
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("URL must use HTTP or HTTPS")
	}
	return nil
}

func validateSecureWebURL(value string) error {
	if err := validateWebURL(value, false); err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, _ := url.Parse(strings.TrimSpace(value))
	if strings.EqualFold(parsed.Scheme, "https") {
		return nil
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	if address := net.ParseIP(host); address != nil && address.IsLoopback() {
		return nil
	}
	return errors.New("URL must use HTTPS outside local development")
}

func validPrimaryColor(value string) bool {
	return hexColorPattern.MatchString(strings.TrimSpace(value))
}

func validEmailDomain(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return len(value) <= 253 && domainPattern.MatchString(value)
}

func validSMTPHost(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 253 {
		return false
	}
	if net.ParseIP(value) != nil {
		return true
	}
	return hostnamePattern.MatchString(value)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
