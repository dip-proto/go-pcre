//go:build js && wasm

// Package libcwasm provides a minimal libc implementation for WebAssembly.
// It implements only the functions needed by the PCRE2 transpiled code.
package libcwasm

import (
	"unsafe"
)

// TLS represents thread-local storage. In WASM (single-threaded), this is simplified.
type TLS struct {
	allocs []uintptr
}

// NewTLS creates a new TLS instance.
func NewTLS() *TLS {
	return &TLS{
		allocs: make([]uintptr, 0),
	}
}

// Close releases resources associated with the TLS.
func (tls *TLS) Close() {
	for _, ptr := range tls.allocs {
		if ptr != 0 {
			freeMemory(ptr)
		}
	}
	tls.allocs = nil
}

// Alloc allocates memory on the TLS stack (simplified for WASM).
func (tls *TLS) Alloc(size int) uintptr {
	return Xmalloc(tls, Size_t(size))
}

// Free frees memory allocated with Alloc.
func (tls *TLS) Free(size int) {
}

var memoryPool = make(map[uintptr][]byte)

func allocMemory(size uintptr) uintptr {
	if size == 0 {
		return 0
	}
	buf := make([]byte, size)
	ptr := uintptr(unsafe.Pointer(&buf[0]))
	memoryPool[ptr] = buf
	return ptr
}

func freeMemory(ptr uintptr) {
	delete(memoryPool, ptr)
}

// Xmalloc allocates memory of the given size.
func Xmalloc(tls *TLS, size Size_t) uintptr {
	ptr := allocMemory(uintptr(size))
	if tls != nil && ptr != 0 {
		tls.allocs = append(tls.allocs, ptr)
	}
	return ptr
}

// Xfree frees previously allocated memory.
func Xfree(tls *TLS, ptr uintptr) {
	if ptr == 0 {
		return
	}
	freeMemory(ptr)
	if tls != nil {
		for i, p := range tls.allocs {
			if p == ptr {
				tls.allocs = append(tls.allocs[:i], tls.allocs[i+1:]...)
				break
			}
		}
	}
}

// Xrealloc reallocates memory.
func Xrealloc(tls *TLS, ptr uintptr, size Size_t) uintptr {
	if ptr == 0 {
		return Xmalloc(tls, size)
	}
	if size == 0 {
		Xfree(tls, ptr)
		return 0
	}

	newPtr := Xmalloc(tls, size)
	if newPtr == 0 {
		return 0
	}

	if oldBuf, ok := memoryPool[ptr]; ok {
		newBuf := memoryPool[newPtr]
		copyLen := len(oldBuf)
		if int(size) < copyLen {
			copyLen = int(size)
		}
		copy(newBuf, oldBuf[:copyLen])
	}

	Xfree(tls, ptr)
	return newPtr
}

// Xcalloc allocates zero-initialized memory.
func Xcalloc(tls *TLS, n, size Size_t) uintptr {
	total := Size_t(n * size)
	ptr := Xmalloc(tls, total)
	if ptr != 0 {
		Xmemset(tls, ptr, 0, total)
	}
	return ptr
}

// CString converts a Go string to a C string (null-terminated).
func CString(s string) (uintptr, error) {
	n := len(s)
	ptr := allocMemory(uintptr(n + 1))
	if ptr == 0 {
		return 0, nil
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n+1)
	copy(buf, s)
	buf[n] = 0
	return ptr, nil
}

// GoString converts a C string to a Go string.
func GoString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var length int
	for {
		if *(*byte)(unsafe.Pointer(ptr + uintptr(length))) == 0 {
			break
		}
		length++
		if length > 1<<20 {
			break
		}
	}
	if length == 0 {
		return ""
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)
	return string(buf)
}

// GoBytes converts a C buffer to Go bytes.
func GoBytes(ptr uintptr, length int) []byte {
	if ptr == 0 || length <= 0 {
		return nil
	}
	buf := make([]byte, length)
	src := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)
	copy(buf, src)
	return buf
}

// Size_t is the Go equivalent of C's size_t.
type Size_t = uint64

// Xmemcpy copies n bytes from src to dest.
func Xmemcpy(tls *TLS, dest, src uintptr, n Size_t) uintptr {
	if n == 0 || dest == 0 || src == 0 {
		return dest
	}
	d := unsafe.Slice((*byte)(unsafe.Pointer(dest)), n)
	s := unsafe.Slice((*byte)(unsafe.Pointer(src)), n)
	copy(d, s)
	return dest
}

// Xmemmove copies n bytes from src to dest (handles overlapping).
func Xmemmove(tls *TLS, dest, src uintptr, n Size_t) uintptr {
	if n == 0 || dest == 0 || src == 0 {
		return dest
	}
	d := unsafe.Slice((*byte)(unsafe.Pointer(dest)), n)
	s := unsafe.Slice((*byte)(unsafe.Pointer(src)), n)
	copy(d, s)
	return dest
}

// Xmemset sets n bytes of dest to the value c.
func Xmemset(tls *TLS, dest uintptr, c int32, n Size_t) uintptr {
	if n == 0 || dest == 0 {
		return dest
	}
	b := byte(c)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(dest)), n)
	for i := range buf {
		buf[i] = b
	}
	return dest
}

// Xmemcmp compares two memory regions.
func Xmemcmp(tls *TLS, s1, s2 uintptr, n Size_t) int32 {
	if n == 0 {
		return 0
	}
	b1 := unsafe.Slice((*byte)(unsafe.Pointer(s1)), n)
	b2 := unsafe.Slice((*byte)(unsafe.Pointer(s2)), n)
	for i := Size_t(0); i < n; i++ {
		if b1[i] < b2[i] {
			return -1
		}
		if b1[i] > b2[i] {
			return 1
		}
	}
	return 0
}

// Xmemchr searches for a byte in memory.
func Xmemchr(tls *TLS, s uintptr, c int32, n Size_t) uintptr {
	if s == 0 || n == 0 {
		return 0
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(s)), n)
	b := byte(c)
	for i := Size_t(0); i < n; i++ {
		if buf[i] == b {
			return s + uintptr(i)
		}
	}
	return 0
}

// Xstrlen returns the length of a C string.
func Xstrlen(tls *TLS, s uintptr) Size_t {
	if s == 0 {
		return 0
	}
	var n Size_t
	for *(*byte)(unsafe.Pointer(s + uintptr(n))) != 0 {
		n++
	}
	return n
}

// Xstrchr finds the first occurrence of c in s.
func Xstrchr(tls *TLS, s uintptr, c int32) uintptr {
	if s == 0 {
		return 0
	}
	b := byte(c)
	for {
		ch := *(*byte)(unsafe.Pointer(s))
		if ch == b {
			return s
		}
		if ch == 0 {
			return 0
		}
		s++
	}
}

// Xstrcmp compares two strings.
func Xstrcmp(tls *TLS, s1, s2 uintptr) int32 {
	for {
		c1 := *(*byte)(unsafe.Pointer(s1))
		c2 := *(*byte)(unsafe.Pointer(s2))
		if c1 != c2 {
			if c1 < c2 {
				return -1
			}
			return 1
		}
		if c1 == 0 {
			return 0
		}
		s1++
		s2++
	}
}

// Xtolower converts a character to lowercase.
func Xtolower(tls *TLS, c int32) int32 {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// Xtoupper converts a character to uppercase.
func Xtoupper(tls *TLS, c int32) int32 {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

const (
	_ISupper  = 1 << 0
	_ISlower  = 1 << 1
	_ISalpha  = 1 << 2
	_ISdigit  = 1 << 3
	_ISxdigit = 1 << 4
	_ISspace  = 1 << 5
	_ISprint  = 1 << 6
	_ISgraph  = 1 << 7
	_ISblank  = 1 << 8
	_IScntrl  = 1 << 9
	_ISpunct  = 1 << 10
	_ISalnum  = 1 << 11
)

var ctypeTable [384]uint16

func init() {
	for i := 0; i < 256; i++ {
		c := i
		var flags uint16

		if c >= 'A' && c <= 'Z' {
			flags |= _ISupper | _ISalpha | _ISalnum | _ISprint | _ISgraph
		}
		if c >= 'a' && c <= 'z' {
			flags |= _ISlower | _ISalpha | _ISalnum | _ISprint | _ISgraph
		}
		if c >= '0' && c <= '9' {
			flags |= _ISdigit | _ISxdigit | _ISalnum | _ISprint | _ISgraph
		}
		if (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f') {
			flags |= _ISxdigit
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v' {
			flags |= _ISspace
		}
		if c == ' ' || c == '\t' {
			flags |= _ISblank
		}
		if c >= 0x20 && c <= 0x7e {
			flags |= _ISprint
		}
		if c >= 0x21 && c <= 0x7e {
			flags |= _ISgraph
		}
		if c < 0x20 || c == 0x7f {
			flags |= _IScntrl
		}
		if (c >= 0x21 && c <= 0x2f) || (c >= 0x3a && c <= 0x40) ||
			(c >= 0x5b && c <= 0x60) || (c >= 0x7b && c <= 0x7e) {
			flags |= _ISpunct
		}

		ctypeTable[128+i] = flags
	}
}

var ctypeTablePtr = uintptr(unsafe.Pointer(&ctypeTable[128]))

// X__ctype_b_loc returns a pointer to the character type table.
func X__ctype_b_loc(tls *TLS) uintptr {
	return uintptr(unsafe.Pointer(&ctypeTablePtr))
}

// Bool32 converts a bool to int32 (0 or 1).
func Bool32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// BoolInt32 converts bool to int32 (0 or 1).
func BoolInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// Uint32 converts to uint32.
func Uint32(v interface{}) uint32 {
	switch x := v.(type) {
	case int:
		return uint32(x)
	case int32:
		return uint32(x)
	case uint32:
		return x
	case int64:
		return uint32(x)
	case uint64:
		return uint32(x)
	default:
		return 0
	}
}

// Uint32FromInt32 converts int32 to uint32.
func Uint32FromInt32(v int32) uint32 {
	return uint32(v)
}

// Uint32FromUint8 converts uint8 to uint32.
func Uint32FromUint8(v uint8) uint32 {
	return uint32(v)
}

// Uint8FromInt32 converts int32 to uint8.
func Uint8FromInt32(v int32) uint8 {
	return uint8(v)
}

// Uint8FromUint32 converts uint32 to uint8.
func Uint8FromUint32(v uint32) uint8 {
	return uint8(v)
}

// UintptrFromInt32 converts int32 to uintptr.
func UintptrFromInt32(v int32) uintptr {
	return uintptr(v)
}

// Int32FromUint32 converts uint32 to int32.
func Int32FromUint32(v uint32) int32 {
	return int32(v)
}

// CplInt32 returns the bitwise complement of an int32.
func CplInt32(v int32) int32 {
	return ^v
}

// CplUint32 returns the bitwise complement of a uint32.
func CplUint32(v uint32) uint32 {
	return ^v
}

// CplUint64 returns the bitwise complement of a uint64.
func CplUint64(v uint64) uint64 {
	return ^v
}

// PostIncInt32 increments *p and returns the old value.
func PostIncInt32(p *int32, d int32) int32 {
	old := *p
	*p += d
	return old
}

// PostIncUint32 increments *p and returns the old value.
func PostIncUint32(p *uint32, d uint32) uint32 {
	old := *p
	*p += d
	return old
}

// PostIncUint64 increments *p and returns the old value.
func PostIncUint64(p *uint64, d uint64) uint64 {
	old := *p
	*p += d
	return old
}

// PostIncUintptr increments *p and returns the old value.
func PostIncUintptr(p *uintptr, d int) uintptr {
	old := *p
	*p += uintptr(d)
	return old
}

// PostDecInt32 decrements *p and returns the old value.
func PostDecInt32(p *int32, d int32) int32 {
	old := *p
	*p -= d
	return old
}

// PostDecUint32 decrements *p and returns the old value.
func PostDecUint32(p *uint32, d uint32) uint32 {
	old := *p
	*p -= d
	return old
}

// PostDecUintptr decrements *p and returns the old value.
func PostDecUintptr(p *uintptr, d int) uintptr {
	old := *p
	*p -= uintptr(d)
	return old
}

// PreIncInt32 increments *p and returns the new value.
func PreIncInt32(p *int32, d int32) int32 {
	*p += d
	return *p
}

// PreIncUint32 increments *p and returns the new value.
func PreIncUint32(p *uint32, d uint32) uint32 {
	*p += d
	return *p
}

// PreIncUintptr increments *p and returns the new value.
func PreIncUintptr(p *uintptr, d int) uintptr {
	*p += uintptr(d)
	return *p
}

// PreDecInt32 decrements *p and returns the new value.
func PreDecInt32(p *int32, d int32) int32 {
	*p -= d
	return *p
}

// PreDecUint32 decrements *p and returns the new value.
func PreDecUint32(p *uint32, d uint32) uint32 {
	*p -= d
	return *p
}

// PreDecUint64 decrements *p and returns the new value.
func PreDecUint64(p *uint64, d uint64) uint64 {
	*p -= d
	return *p
}

// AssignInt32 assigns v to *p and returns v.
func AssignInt32(p *int32, v int32) int32 {
	*p = v
	return v
}

// AssignUint32 assigns v to *p and returns v.
func AssignUint32(p *uint32, v uint32) uint32 {
	*p = v
	return v
}

// AssignUint64 assigns v to *p and returns v.
func AssignUint64(p *uint64, v uint64) uint64 {
	*p = v
	return v
}

// AssignUint8 assigns v to *p and returns v.
func AssignUint8(p *uint8, v uint8) uint8 {
	*p = v
	return v
}

// AssignUintptr assigns v to *p and returns v.
func AssignUintptr(p *uintptr, v uintptr) uintptr {
	*p = v
	return v
}

// AssignAddInt32 adds v to *p and returns the new value.
func AssignAddInt32(p *int32, v int32) int32 {
	*p += v
	return *p
}

// AssignPtrInt32 assigns v to the int32 at ptr.
func AssignPtrInt32(ptr uintptr, v int32) int32 {
	*(*int32)(unsafe.Pointer(ptr)) = v
	return v
}

// AssignPtrUint8 assigns v to the uint8 at ptr.
func AssignPtrUint8(ptr uintptr, v uint8) uint8 {
	*(*uint8)(unsafe.Pointer(ptr)) = v
	return v
}

// AssignPtrUint16 assigns v to the uint16 at ptr.
func AssignPtrUint16(ptr uintptr, v uint16) uint16 {
	*(*uint16)(unsafe.Pointer(ptr)) = v
	return v
}

// AssignPtrUint32 assigns v to the uint32 at ptr.
func AssignPtrUint32(ptr uintptr, v uint32) uint32 {
	*(*uint32)(unsafe.Pointer(ptr)) = v
	return v
}

// AssignPtrUint64 assigns v to the uint64 at ptr.
func AssignPtrUint64(ptr uintptr, v uint64) uint64 {
	*(*uint64)(unsafe.Pointer(ptr)) = v
	return v
}

// AssignPtrUintptr assigns v to the uintptr at ptr.
func AssignPtrUintptr(ptr uintptr, v uintptr) uintptr {
	*(*uintptr)(unsafe.Pointer(ptr)) = v
	return v
}
