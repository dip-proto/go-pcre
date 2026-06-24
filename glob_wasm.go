//go:build wasm

package pcre

import (
	"errors"
	"unsafe"

	"go.elara.ws/pcre/lib"
	libc "go.elara.ws/pcre/libcwasm"
)

// ErrGlobNotSupported is returned when Glob is called in a WASM environment.
var ErrGlobNotSupported = errors.New("filesystem glob not supported in WebAssembly")

// ConvertGlob converts the given glob into a
// pcre regular expression, and then returns
// the result.
func ConvertGlob(glob string) (string, error) {
	tls := libc.NewTLS()
	defer tls.Close()

	cGlob, err := libc.CString(glob)
	if err != nil {
		return "", err
	}
	defer libc.Xfree(tls, cGlob)
	cGlobLen := lib.Tsize_t(len(glob))

	outPtr := uintptr(0)
	cOutPtr := uintptr(unsafe.Pointer(&outPtr))

	outLen := lib.Tsize_t(0)
	cOutLen := uintptr(unsafe.Pointer(&outLen))

	ret := lib.Xpcre2_pattern_convert_8(
		tls,
		cGlob,
		cGlobLen,
		lib.DPCRE2_CONVERT_GLOB,
		cOutPtr,
		cOutLen,
		0,
	)
	if ret != 0 {
		return "", codeToError(tls, ret)
	}
	defer lib.Xpcre2_converted_pattern_free_8(tls, outPtr)

	out := unsafe.Slice((*byte)(unsafe.Pointer(outPtr)), outLen)
	return string(out), nil
}

// CompileGlob is a convenience function that converts
// a glob to a pcre regular expression and then compiles
// it.
func CompileGlob(glob string) (*Regexp, error) {
	pattern, err := ConvertGlob(glob)
	if err != nil {
		return nil, err
	}
	return Compile(pattern)
}

// Glob is not supported in WebAssembly environments as it requires
// filesystem access. Use ConvertGlob and CompileGlob instead to
// work with glob patterns.
func Glob(glob string) ([]string, error) {
	return nil, ErrGlobNotSupported
}

// hasGlobChars checks if the string has any
// characters that are part of a glob.
func hasGlobChars(s string) bool {
	for _, c := range s {
		if c == '*' || c == '[' || c == ']' || c == '?' {
			return true
		}
	}
	return false
}
