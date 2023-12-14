package provider

import (
	"bufio"
	"errors"
	"github.com/kataras/iris/v12/context"
	"net"
	"net/http"
)

// ResponseWriter is the basic response writer,
// it writes directly to the underline http.ResponseWriter
type responseWriter struct {
	http.ResponseWriter

	statusCode int // the saved status code which will be used from the cache service
	// statusCodeSent bool // reply header has been (logically) written | no needed any more as we have a variable to catch total len of written bytes
	written int // the total size of bytes were written
	// yes only one callback, we need simplicity here because on FireStatusCode the beforeFlush events should NOT be cleared
	// but the response is cleared.
	// Sometimes is useful to keep the event,
	// so we keep one func only and let the user decide when he/she wants to override it with an empty func before the FireStatusCode (context's behavior)
	beforeFlush func()
}

const (
	defaultStatusCode = http.StatusOK
	// NoWritten !=-1 => when nothing written before
	NoWritten = -1
	// StatusCodeWritten != 0 =>  when only status code written
	StatusCodeWritten = 0
)

// Naive returns the simple, underline and original http.ResponseWriter
// that backends this response writer.
func (w *responseWriter) Naive() http.ResponseWriter {
	return w.ResponseWriter
}

// BeginResponse receives an http.ResponseWriter
// and initialize or reset the response writer's field's values.
func (w *responseWriter) BeginResponse(underline http.ResponseWriter) {
	w.beforeFlush = nil
	w.written = NoWritten
	w.statusCode = defaultStatusCode
	w.SetWriter(underline)
}

// SetWriter sets the underline http.ResponseWriter
// that this responseWriter should write on.
func (w *responseWriter) SetWriter(underline http.ResponseWriter) {
	w.ResponseWriter = underline
}

// EndResponse is the last function which is called right before the server sent the final response.
//
// Here is the place which we can make the last checks or do a cleanup.
func (w *responseWriter) EndResponse() {
	// todo
}

// Reset clears headers, sets the status code to 200
// and clears the cached body.
//
// Implements the `ResponseWriterReseter`.
func (w *responseWriter) Reset() bool {
	if w.written > 0 {
		return false // if already written we can't reset this type of response writer.
	}

	h := w.Header()
	for k := range h {
		h[k] = nil
	}

	w.written = NoWritten
	w.statusCode = defaultStatusCode
	return true
}

// SetWritten sets manually a value for written, it can be
// NoWritten(-1) or StatusCodeWritten(0), > 0 means body length which is useless here.
func (w *responseWriter) SetWritten(n int) {
	if n >= NoWritten && n <= StatusCodeWritten {
		w.written = n
	}
}

// Written should returns the total length of bytes that were being written to the client.
// In addition iris provides some variables to help low-level actions:
// NoWritten, means that nothing were written yet and the response writer is still live.
// StatusCodeWritten, means that status code were written but no other bytes are written to the client, response writer may closed.
// > 0 means that the reply was written and it's the total number of bytes were written.
func (w *responseWriter) Written() int {
	return w.written
}

// WriteHeader sends an HTTP response header with status code.
// If WriteHeader is not called explicitly, the first call to Write
// will trigger an implicit WriteHeader(http.StatusOK).
// Thus explicit calls to WriteHeader are mainly used to
// send error codes.
func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *responseWriter) tryWriteHeader() {
	if w.written == NoWritten { // before write, once.
		w.written = StatusCodeWritten
		w.ResponseWriter.WriteHeader(w.statusCode)
	}
}

// IsHijacked reports whether this response writer's connection is hijacked.
func (w *responseWriter) IsHijacked() bool {
	// Note:
	// A zero-byte `ResponseWriter.Write` on a hijacked connection will
	// return `http.ErrHijacked` without any other side effects.
	_, err := w.ResponseWriter.Write(nil)
	return err == http.ErrHijacked
}

// Write writes to the client
// If WriteHeader has not yet been called, Write calls
// WriteHeader(http.StatusOK) before writing the data. If the Header
// does not contain a Content-Type line, Write adds a Content-Type set
// to the result of passing the initial 512 bytes of written data to
// DetectContentType.
//
// Depending on the HTTP protocol version and the client, calling
// Write or WriteHeader may prevent future reads on the
// Request.Body. For HTTP/1.x requests, handlers should read any
// needed request body data before writing the response. Once the
// headers have been flushed (due to either an explicit Flusher.Flush
// call or writing enough data to trigger a flush), the request body
// may be unavailable. For HTTP/2 requests, the Go HTTP server permits
// handlers to continue to read the request body while concurrently
// writing the response. However, such behavior may not be supported
// by all HTTP/2 clients. Handlers should read before writing if
// possible to maximize compatibility.
func (w *responseWriter) Write(contents []byte) (int, error) {
	w.tryWriteHeader()
	n, err := w.ResponseWriter.Write(contents)
	w.written += n
	return n, err
}

// StatusCode returns the status code header value
func (w *responseWriter) StatusCode() int {
	return w.statusCode
}

func (w *responseWriter) GetBeforeFlush() func() {
	return w.beforeFlush
}

// SetBeforeFlush registers the unique callback which called exactly before the response is flushed to the client
func (w *responseWriter) SetBeforeFlush(cb func()) {
	w.beforeFlush = cb
}

func (w *responseWriter) FlushResponse() {
	if w.beforeFlush != nil {
		w.beforeFlush()
	}

	w.tryWriteHeader()
}

// Clone returns a clone of this response writer
// it copies the header, status code, headers and the beforeFlush finally  returns a new ResponseRecorder.
func (w *responseWriter) Clone() context.ResponseWriter {
	wc := &responseWriter{}
	wc.ResponseWriter = w.ResponseWriter
	wc.statusCode = w.statusCode
	wc.beforeFlush = w.beforeFlush
	wc.written = w.written
	return wc
}

// CopyTo writes a response writer (temp: status code, headers and body) to another response writer.
func (w *responseWriter) CopyTo(to context.ResponseWriter) {
	// set the status code, failure status code are first class
	if w.statusCode >= 400 {
		to.WriteHeader(w.statusCode)
	}

	// append the headers
	for k, values := range w.Header() {
		for _, v := range values {
			if to.Header().Get(v) == "" {
				to.Header().Add(k, v)
			}
		}
	}
	// the body is not copied, this writer doesn't support recording
}

// ErrHijackNotSupported is returned by the Hijack method to
// indicate that Hijack feature is not available.
var ErrHijackNotSupported = errors.New("hijack is not supported by this ResponseWriter")

// Hijack lets the caller take over the connection.
// After a call to Hijack(), the HTTP server library
// will not do anything else with the connection.
//
// It becomes the caller's responsibility to manage
// and close the connection.
//
// The returned net.Conn may have read or write deadlines
// already set, depending on the configuration of the
// Server. It is the caller's responsibility to set
// or clear those deadlines as needed.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, isHijacker := w.ResponseWriter.(http.Hijacker); isHijacker {
		w.written = StatusCodeWritten
		return h.Hijack()
	}

	return nil, nil, ErrHijackNotSupported
}

// Flusher indicates if `Flush` is supported by the client.
//
// The default HTTP/1.x and HTTP/2 ResponseWriter implementations
// support Flusher, but ResponseWriter wrappers may not. Handlers
// should always test for this ability at runtime.
//
// Note that even for ResponseWriters that support Flush,
// if the client is connected through an HTTP proxy,
// the buffered data may not reach the client until the response
// completes.
func (w *responseWriter) Flusher() (http.Flusher, bool) {
	flusher, canFlush := w.ResponseWriter.(http.Flusher)
	return flusher, canFlush
}

// Flush sends any buffered data to the client.
func (w *responseWriter) Flush() {
	if flusher, ok := w.Flusher(); ok {
		// Flow: WriteHeader -> Flush -> Write -> Write -> Write....
		w.tryWriteHeader()

		flusher.Flush()
	}
}
