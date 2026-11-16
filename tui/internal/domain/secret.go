package domain

const redacted = "[redacted]"

// Secret hides its value from fmt, JSON and TOML encoders; Reveal is the only reader.
// fmt skips Stringer/GoString/TextMarshaler for values behind unexported struct fields and string(s) skips
// redaction, so keep a Secret in exported fields of exported types only.
type Secret string

func (Secret) String() string               { return redacted }
func (Secret) GoString() string             { return redacted }
func (Secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }
func (s Secret) Reveal() string             { return string(s) }
