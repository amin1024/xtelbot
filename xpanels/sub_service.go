package xpanels

import (
	"fmt"
	"io"
	"strings"
)

// SubRenovator receive an original sub-content (io.Reader) and returns a modified version of it
type SubRenovator interface {
	Renovate(reader io.Reader, uid string) (string, error)
}

// -----------------------------------------------------------------

type RuleRenovator struct {
	groupRules map[string][]RenovateRule
}

func (r *RuleRenovator) Renovate(subContent io.Reader, _ string) (string, error) {
	var result []string

	data, err := io.ReadAll(subContent)
	if err != nil {
		return "", err
	}
	// Parse content assuming every V2ray config is being seperated by '\n'
	lines := strings.Split(string(data), "\n")
	if len(lines) < 10 { // Hack around it, todo
		return "", fmt.Errorf("is a sub content with %d len valid", len(data))
	}

	// renovate every v2ray config found in the sub
	for _, line := range lines {
		if len(line) < 10 {
			continue
		}
		v2rayUri := strings.TrimSpace(line)
		if strings.HasPrefix(v2rayUri, "#") {
			result = append(result, line)
			continue
		}
		split := strings.Split(v2rayUri, "#")
		if len(split) != 2 {
			continue
		}
		rules, ok := r.groupRules[split[1]] // split[1] gives us the #remark in uri
		if ok {
			v2rayUri = r.renovateV2rayConfig(v2rayUri, rules)
		}
		result = append(result, v2rayUri)
	}
	if len(result) == 0 {
		return "", fmt.Errorf("sub-content without any valid v2ray config")
	}

	return strings.Join(result, "\n"), nil
}

// renovateV2rayConfig receives a single v2ray uri and modify it according to the rules.
// Example uri vless://UUID@SERVER:PORT?security=tls&sni=SNI&type=grpc&serviceName=GRPC-NAME&#REMARK
// We want to replace the #REMARK and :PORT, Therefore, two rules needed to be passed to this method.
func (r *RuleRenovator) renovateV2rayConfig(v2rayUri string, rules []RenovateRule) string {
	for _, rule := range rules {
		if rule.Ignore {
			return "# IGNORED #"
		}
		v2rayUri = strings.ReplaceAll(v2rayUri, rule.OldValue, rule.NewValue)
	}
	return v2rayUri
}

// -----------------------------------------------------------------

type RenovateFromFile struct {
	content string
}

func NewRenovatorFromFile(content string) RenovateFromFile {
	return RenovateFromFile{content: content}
}

// Renovate returns the content of file without modifying it
func (s RenovateFromFile) Renovate(_ io.Reader, uid string) (string, error) {
	return strings.ReplaceAll(s.content, "USERUUID", uid), nil
}
