package synco

import (
	"io"
	"io/ioutil"
	"net/mail"
	"strings"
	"testing"
)

var mime1 = "Message-ID: 12a34\r\nTo: a@a.com\r\nCc: b@b.com\r\nSubject: mySubject\r\nDate: Tue, 1 Jul 2003 10:52:37 +0200\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nhello this is text"

var mime2 = "Message-ID: 12a34\r\nTo: a@a.com\r\nCc: b@b.com\r\nSubject: mySubject\r\nDate: Tue, 1 Jul 2003 10:52:37 +0200\r\nContent-Type: multipart/alternative; boundary=f46d043c80b4d7a9b904bce66840\r\n\r\n--f46d043c80b4d7a9b904bce66840\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nhello this is text\r\n--f46d043c80b4d7a9b904bce66840\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n<p>E=3Dmc^2</p>\r\n--f46d043c80b4d7a9b904bce66840\r\n"

func Test_HTMLBody(t *testing.T) {
	body, err := HTMLBody(stringToMessage(mime1))
	if err != nil {
		t.Errorf("mime1 should have no html body")
	}

	body, err = HTMLBody(stringToMessage(mime2))
	expectBodyEquals(t, body, err, "<p>E=mc^2</p>", "mime2 HTML")
}


func Test_TextBody(t *testing.T) {
	body, err := TextBody(stringToMessage(mime1))
	expectBodyEquals(t, body, err, "hello this is text", "mime1 text")

	body, err = TextBody(stringToMessage(mime2))
	expectBodyEquals(t, body, err, "hello this is text", "mime2 text")
}

func expectBodyEquals(t *testing.T, bodyString string, err error, expected string, label string) {
	if err != nil {
		t.Fatalf("%s has non-nil error: %s\n", label, err)
	}
	if bodyString != expected {
		t.Errorf("%s should have body '%s', got '%s'", label, expected, bodyString)
	}
}

func stringToMessage(mimestring string) *mail.Message {
	msg, err := mail.ReadMessage(strings.NewReader(mimestring))
	if err != nil {
		panic("err")
	}
	return msg
}

func readerToString(rdr io.Reader) string {
	if rdr == nil {
		panic("rdr is nil")
	}
	b, err := ioutil.ReadAll(rdr)
	if err != nil {
		return ""
	}
	return string(b)
}