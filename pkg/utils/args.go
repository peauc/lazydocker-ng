package utils

import (
	"errors"
	"runtime"
	"strings"
)

var (
	ErrUnterminatedQuote = errors.New("unterminated quote")
	ErrTrailingEscape    = errors.New("trailing backslash")
)

// SplitArgs splits a command line into arguments with the rules lazydocker's
// command templates have always used (those of mgutz/str.ToArgv): spaces and
// tabs separate arguments, ' and " group, and adjacent pieces join. On unix a
// backslash escapes the next character; on windows it is literal, except that
// a backslash right before a " is dropped. Where ToArgv panicked (an
// unterminated quote, a trailing backslash on unix) SplitArgs returns an error.
func SplitArgs(s string) ([]string, error) {
	return splitArgs(s, runtime.GOOS == "windows")
}

func splitArgs(s string, windows bool) ([]string, error) {
	const (
		outOfArg = iota
		inArg
		inQuote
	)
	state := outOfArg
	var quote byte
	var current strings.Builder
	args := []string{}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\'':
			switch state {
			case outOfArg, inArg:
				state = inQuote
				quote = c
			case inQuote:
				if c == quote {
					state = inArg
				} else {
					current.WriteByte(c)
				}
			}

		case ' ', '\t':
			switch state {
			case inArg:
				args = append(args, current.String())
				current.Reset()
				state = outOfArg
			case inQuote:
				current.WriteByte(c)
			}

		case '\\':
			if state == outOfArg {
				state = inArg
			}
			switch {
			case windows:
				if i+1 >= len(s) || s[i+1] != '"' {
					current.WriteByte(c)
				}
			case i+1 >= len(s):
				return nil, ErrTrailingEscape
			default:
				i++
				current.WriteByte(s[i])
			}

		default:
			if state == outOfArg {
				state = inArg
			}
			current.WriteByte(c)
		}
	}

	switch state {
	case inArg:
		args = append(args, current.String())
	case inQuote:
		return nil, ErrUnterminatedQuote
	}
	return args, nil
}

// QuoteArg quotes s so that SplitArgs reads it back as exactly one argument.
// Arguments without spaces, tabs, quotes or backslashes are returned as is.
func QuoteArg(s string) string {
	return quoteArg(s, runtime.GOOS == "windows")
}

func quoteArg(s string, windows bool) string {
	if s != "" && !strings.ContainsAny(s, " \t'\"\\") {
		return s
	}

	if !windows {
		s = strings.ReplaceAll(s, `\`, `\\`)
	}
	// a ' cannot appear inside '...': close, add it inside "...", reopen
	s = strings.ReplaceAll(s, `'`, `'"'"'`)
	if windows {
		// windows drops a \ written right before a ": end the quoted piece
		// between them so the \ is followed by ' instead
		s = strings.ReplaceAll(s, `\"`, `\''"`)
	}
	return "'" + s + "'"
}

// JoinArgs quotes each argument with QuoteArg and joins them with spaces.
func JoinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = QuoteArg(arg)
	}
	return strings.Join(quoted, " ")
}
