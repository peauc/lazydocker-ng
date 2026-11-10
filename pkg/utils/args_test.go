package utils

import (
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

type splitResult struct {
	args []string
	err  error
}

func ok(args ...string) splitResult { return splitResult{args: append([]string{}, args...)} }
func fails(err error) splitResult   { return splitResult{err: err} }

// TestSplitArgs pins the rules of mgutz/str.ToArgv, which this replaces, for
// unix and windows; where ToArgv panicked, SplitArgs returns an error.
func TestSplitArgs(t *testing.T) {
	scenarios := []struct {
		in            string
		unix, windows splitResult
	}{
		{`docker ps -a`, ok("docker", "ps", "-a"), ok("docker", "ps", "-a")},
		{"a  b\tc", ok("a", "b", "c"), ok("a", "b", "c")},
		{`"a b" c`, ok("a b", "c"), ok("a b", "c")},
		{`'a "b" c'`, ok(`a "b" c`), ok(`a "b" c`)},
		{`foo"bar baz"`, ok("foobar baz"), ok("foobar baz")},
		{`""`, ok(""), ok("")},
		{``, ok(), ok()},
		{`   `, ok(), ok()},
		{"a\nb", ok("a\nb"), ok("a\nb")},
		{`a\ b`, ok("a b"), ok(`a\`, "b")},
		{`C:\Program Files\x.exe`, ok("C:Program", "Filesx.exe"), ok(`C:\Program`, `Files\x.exe`)},
		{
			`"C:\Program Files\x.exe" -f "C:\my dir\dc.yml"`,
			ok("C:Program Filesx.exe", "-f", "C:my dirdc.yml"),
			ok(`C:\Program Files\x.exe`, "-f", `C:\my dir\dc.yml`),
		},
		{`"C:\dir\" x`, fails(ErrUnterminatedQuote), ok(`C:\dir`, "x")},
		{`echo "hi`, fails(ErrUnterminatedQuote), fails(ErrUnterminatedQuote)},
		{`it's`, fails(ErrUnterminatedQuote), fails(ErrUnterminatedQuote)},
		{`a\`, fails(ErrTrailingEscape), ok(`a\`)},
	}

	for _, s := range scenarios {
		for _, mode := range []struct {
			windows bool
			want    splitResult
		}{{false, s.unix}, {true, s.windows}} {
			args, err := splitArgs(s.in, mode.windows)
			if mode.want.err != nil {
				assert.ErrorIs(t, err, mode.want.err, "in=%q windows=%v", s.in, mode.windows)
				continue
			}
			assert.NoError(t, err, "in=%q windows=%v", s.in, mode.windows)
			assert.Equal(t, mode.want.args, args, "in=%q windows=%v", s.in, mode.windows)
		}
	}
}

func TestQuoteArgRoundTrips(t *testing.T) {
	corpus := []string{
		"", "plain", "two words", "it's", `say "hi"`, `both ' and "`, `back\slash`, `trailing\`,
		`C:\my dir\`, "\ttab", "-p", "ünïcødé", "$HOME", `a\"b`, `\"`, `\\"`, `x\'y`, "line1\nline2",
	}
	for _, windows := range []bool{false, true} {
		for _, s := range corpus {
			args, err := splitArgs(quoteArg(s, windows), windows)
			assert.NoError(t, err, "s=%q windows=%v", s, windows)
			assert.Equal(t, []string{s}, args, "s=%q windows=%v quoted=%q", s, windows, quoteArg(s, windows))
		}

		roundTrips := func(s string) bool {
			args, err := splitArgs(quoteArg(s, windows), windows)
			return err == nil && len(args) == 1 && args[0] == s
		}
		assert.NoError(t, quick.Check(roundTrips, &quick.Config{MaxCount: 2000}), "windows=%v", windows)
	}
}

func TestQuoteArgLeavesSimpleArgs(t *testing.T) {
	assert.Equal(t, "one.yml", quoteArg("one.yml", false))
	assert.Equal(t, "one.yml", quoteArg("one.yml", true))
	assert.Equal(t, `'x -f evil.yml'`, quoteArg("x -f evil.yml", false))
}

func TestJoinArgs(t *testing.T) {
	joined := JoinArgs([]string{"-f", "my file.yml", "-p", "proj"})

	args, err := SplitArgs(joined)
	assert.NoError(t, err)
	assert.Equal(t, []string{"-f", "my file.yml", "-p", "proj"}, args)
}
