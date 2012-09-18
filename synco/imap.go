package synco

import "code.google.com/p/go-imap/go1/imap"
import (
	"bytes"
	"fmt"
	"encoding/json"
	"log"
	"net/mail"
	"os"
	"time"
)

type IMAPServer struct {
	Host string
	Port uint16
}

type IMAPAccount struct {
	Username string
	Password string
	Server *IMAPServer
}

type UIDFetchJob struct {
	uids []uint32
}

var	mbox string = "[Gmail]/All Mail"

func PrintMail(acct *IMAPAccount, query string) {
	imap.DefaultLogger = log.New(os.Stdout, "", 0)
//	imap.DefaultLogMask = imap.LogConn | imap.LogRaw

	log.Printf("Running for user '%s' on IMAP server '%s:%d'", acct.Username, acct.Server.Host, acct.Server.Port)

	jobsize := 250

	// Fetch UIDs.
	c := Dial(acct.Server)
	login(c, acct.Username, acct.Password)
	c.Select(mbox, true)
	uids, _ := SearchUIDs(c, query)

	timestarted := time.Now()

	nparts := (len(uids) + jobsize - 1) / jobsize
	jobs := make([]*imap.SeqSet, nparts)
	for i := 0; i < nparts; i++ {
		lo := i * jobsize
		hi_exclusive := (i + 1) * jobsize
		if hi_exclusive >= len(uids) {
			hi_exclusive = len(uids) - 1
			for uids[hi_exclusive] == 0 { // hacky
				hi_exclusive--
			}
		}
		set, _ := imap.NewSeqSet("")
		set.AddNum(uids[lo:hi_exclusive]...)
		jobs[i] = set
	}

	log.Printf("%d UIDs total, %d jobs of size <= %d\n", len(uids), len(jobs), jobsize)

	for _, jobUIDs := range jobs {
		FetchMessages(c, jobUIDs)
	}

	timeelapsed := time.Since(timestarted)
	msecpermessage := timeelapsed.Seconds() / float64(len(uids)) * 1000
	messagespersec := float64(len(uids)) / timeelapsed.Seconds()
	log.Printf("Finished fetching %d messages in %.2fs (%.1fms per message; %.1f messages per second)\n", len(uids), timeelapsed.Seconds(), msecpermessage, messagespersec)

	c.Close(false)
	c.Logout(-1)
}

func SearchUIDs(c *imap.Client, query string) (uids []uint32, err error) {
	cmd, err := c.UIDSearch("X-GM-RAW", fmt.Sprint("\"", query, "\""))
	
	for cmd.InProgress() {
		c.Recv(-1)
		for _, rsp := range cmd.Data {
			uids = rsp.SearchResults()
		}
		cmd.Data = nil
	}
	return
}

func FetchAllUIDs(c *imap.Client) (uids []uint32, err error) {
	maxmessages := 150000
	uids = make([]uint32, maxmessages)

	set, _ := imap.NewSeqSet("1:*")
	cmd, err := c.UIDFetch(set, "RFC822.SIZE")
	
	messagenum := uint32(0)
	for cmd.InProgress() {
		c.Recv(-1)
		for _, rsp := range cmd.Data {
			uid := imap.AsNumber(rsp.MessageInfo().Attrs["UID"])
			uids[messagenum] = uid
		}
		cmd.Data = nil
		messagenum++
	}

	uids = uids[:messagenum]
	return
}


func FetchMessages(c *imap.Client, uidSet *imap.SeqSet) (err error) {
	cmd, err := c.UIDFetch(uidSet, "RFC822")

	for cmd.InProgress() {
		c.Recv(-1)
		for _, rsp := range cmd.Data {
			uid := imap.AsNumber(rsp.MessageInfo().Attrs["UID"])
			mime := imap.AsBytes(rsp.MessageInfo().Attrs["RFC822"])
			if msg, _ := mail.ReadMessage(bytes.NewReader(mime)); msg != nil {
				PrintMessageAsJSON(msg, uid)
			}
		}
		cmd.Data = nil
	}

	return
}

func PrintMessageAsJSON(msg *mail.Message, uid uint32) {
	var msgdata = map[string] string { }

	for headerkey := range msg.Header {
		val := msg.Header.Get(headerkey)
		msgdata[headerkey] = val
	}

	msgdata["imap_uid"] = fmt.Sprintf("%d", uid)

	if b, err := TextBody(msg); err == nil {
		msgdata["text_body"] = b
	}
	if b, err := HTMLBody(msg); err == nil {
		msgdata["html_body"] = b
	}

	o, err := json.Marshal(msgdata)
	if err != nil {
		log.Println("error marshaling message as JSON: ", err.Error()[:100])
	} else {
		fmt.Println(string(o))
	}
	return
}

func Dial(server *IMAPServer) (c *imap.Client) {
	var err error
	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	c, err = imap.DialTLS(addr, nil)
	if err != nil {
		panic(err)
	}
	return c
}

func login(c *imap.Client, user, pass string) (cmd *imap.Command, err error) {
	defer c.SetLogMask(sensitive(c, "LOGIN"))
	return c.Login(user, pass)
}

func sensitive(c *imap.Client, action string) imap.LogMask {
	mask := c.SetLogMask(imap.LogConn)
	hide := imap.LogCmd | imap.LogRaw
	if mask&hide != 0 {
		c.Logln(imap.LogConn, "Raw logging disabled during", action)
	}
	c.SetLogMask(mask &^ hide)
	return mask
}
