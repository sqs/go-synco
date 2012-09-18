package synco

import "github.com/sloonz/go-qprintable"
//import "code.google.com/p/go-charset/charset"
import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func HTMLBody(msg *mail.Message) (body string, err error) {
	return toString(BodyOfType(msg, "text/html"))
}

func TextBody(msg *mail.Message) (body string, err error) {
	return toString(BodyOfType(msg, "text/plain"))
}

func toString(r io.Reader, err error) (string, error) {
	if r == nil || err != nil {
		return "", err
	}
	bodyBytes, err := ioutil.ReadAll(r)
	if err == nil {
//		r, _ := charset.NewReader("UTF-8")
		if utf8.ValidString(string(bodyBytes)) {
			return string(bodyBytes), nil	
		}
	}
	return "", err
}

func BodyOfType(msg *mail.Message, mimetype string) (body io.Reader, err error) {
	typeheader := msg.Header.Get("Content-Type")
	if strings.HasPrefix(typeheader, mimetype) {
		body = msg.Body
	} else if strings.HasPrefix(typeheader, "multipart/") {
		body, err = MultipartBodyOfType(msg, mimetype)
	}
	return
}

func MultipartBodyOfType(msg *mail.Message, mimetype string) (body io.Reader, err error) {
	var mediaType string
	var params map[string]string
	if mediaType, params, err = mime.ParseMediaType(msg.Header.Get("Content-Type")); err != nil {
		return
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		err = errors.New("Content-Type is not multipart")
		return
	}
	body, err = parseMultipart(msg.Body, params["boundary"], mimetype)
	return
}

// from https://github.com/bradfitz/websomtep/blob/master/websomtep.go
func parseMultipart(r io.Reader, boundary string, mimetype string) (body io.Reader, err error) {
	mr := multipart.NewReader(r, boundary)
	for {
		var part *multipart.Part
		part, err = mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}
		partType, partParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if strings.HasPrefix(partType, "multipart/") {
			body, err = parseMultipart(part, partParams["boundary"], mimetype)
			if err != nil {
//				log.Printf("in boundary %q, returning error for multipart child %q: %v", boundary, partParams["boundary"], err)
				return
			}
			continue
		}
		if !strings.HasPrefix(partType, "text/") {
			continue
		}
		if partType == mimetype {
			body, err = decodePartBody(part)
			return
		}
	}
	return
}

func decodePartBody(part *multipart.Part) (body io.Reader, err error) {
	enctype := part.Header.Get("Content-Transfer-Encoding")
	if enctype == "" {
		body = part
		return
	}
	if strings.HasPrefix(enctype, "quoted-printable") {
		body = qprintable.NewDecoder(qprintable.UnixTextEncoding, part)
	} else if enctype == "7bit" {
		
	} else {
		err = errors.New(fmt.Sprintf("unhandled Content-Transfer-Encoding: %s", enctype))
	}
	return
}
