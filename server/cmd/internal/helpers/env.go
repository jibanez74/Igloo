package helpers

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// LoadEnvFile loads KEY=VALUE pairs from path without overwriting existing
// process environment variables.
func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	existing := make(map[string]bool)
	for _, env := range os.Environ() {
		key, _, ok := strings.Cut(env, "=")
		if ok {
			existing[key] = true
		}
	}

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		key, value, ok, err := parseEnvLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		if !ok {
			continue
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	for key, value := range values {
		if existing[key] {
			continue
		}
		err := os.Setenv(key, value)
		if err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	return nil
}

func parseEnvLine(line string) (string, string, bool, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false, nil
	}

	if strings.HasPrefix(trimmed, "export") {
		rest := strings.TrimPrefix(trimmed, "export")
		if rest == "" || unicode.IsSpace([]rune(rest)[0]) {
			trimmed = strings.TrimSpace(rest)
		}
	}

	key, value, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", "", false, fmt.Errorf("invalid env line")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false, fmt.Errorf("missing env key")
	}
	if strings.ContainsFunc(key, unicode.IsSpace) {
		return "", "", false, fmt.Errorf("invalid env key %q", key)
	}

	parsedValue, err := parseEnvValue(value)
	if err != nil {
		return "", "", false, err
	}

	return key, parsedValue, true, nil
}

func parseEnvValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if value[0] == '\'' {
		return parseSingleQuotedEnvValue(value)
	}
	if value[0] == '"' {
		return parseDoubleQuotedEnvValue(value)
	}

	value = stripUnquotedEnvComment(value)
	return strings.TrimSpace(value), nil
}

func parseSingleQuotedEnvValue(value string) (string, error) {
	end := strings.IndexByte(value[1:], '\'')
	if end < 0 {
		return "", fmt.Errorf("unterminated single-quoted env value")
	}
	end++

	remainder := strings.TrimSpace(value[end+1:])
	if remainder != "" && !strings.HasPrefix(remainder, "#") {
		return "", fmt.Errorf("unexpected text after single-quoted env value")
	}

	return value[1:end], nil
}

func parseDoubleQuotedEnvValue(value string) (string, error) {
	var builder strings.Builder
	escaped := false

	for i := 1; i < len(value); i++ {
		ch := value[i]
		if escaped {
			switch ch {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case '"':
				builder.WriteByte('"')
			case '\\':
				builder.WriteByte('\\')
			default:
				builder.WriteByte('\\')
				builder.WriteByte(ch)
			}
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '"':
			remainder := strings.TrimSpace(value[i+1:])
			if remainder != "" && !strings.HasPrefix(remainder, "#") {
				return "", fmt.Errorf("unexpected text after double-quoted env value")
			}
			return builder.String(), nil
		default:
			builder.WriteByte(ch)
		}
	}

	if escaped {
		builder.WriteByte('\\')
	}

	return "", fmt.Errorf("unterminated double-quoted env value")
}

func stripUnquotedEnvComment(value string) string {
	for index, ch := range value {
		if ch != '#' || index == 0 {
			continue
		}

		previous, _ := utf8.DecodeLastRuneInString(value[:index])
		if unicode.IsSpace(previous) {
			return value[:index]
		}
	}
	return value
}
