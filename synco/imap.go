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
	LoUID, HiUID uint32
}

var	mbox string = "[Gmail]/All Mail"

func PrintMail(acct *IMAPAccount, query string) {
	imap.DefaultLogger = log.New(os.Stdout, "", 0)
//	imap.DefaultLogMask = imap.LogConn | imap.LogRaw

	log.Printf("Running for user '%s' on IMAP server '%s:%d'", acct.Username, acct.Server.Host, acct.Server.Port)

	conns := 1
	partsize := 250

	// Fetch UIDs.
	c := Dial(acct.Server)
	login(c, acct.Username, acct.Password)
	c.Select(mbox, true)
	uids, _ := SearchUIDs(c, query)
	c.Close(false)
	c.Logout(-1)

	timestarted := time.Now()

	// Fetch message MIME bodies.
	nparts := (len(uids) + partsize - 1) / partsize
	jobs := make([]UIDFetchJob, nparts)
	for i := 0; i < nparts; i++ {
		lo := i * partsize
		hi_exclusive := (i + 1) * partsize
		if hi_exclusive >= len(uids) {
			hi_exclusive = len(uids) - 1
			for uids[hi_exclusive] == 0 { // hacky
				hi_exclusive--
			}
		}
		loUID := uids[lo]
		hiUID := uids[hi_exclusive]
		job := UIDFetchJob{ loUID, hiUID }
		jobs[i] = job
	}

	log.Printf("%d UIDs total, %d jobs of size <= %d\n", len(uids), len(jobs), partsize)

	// open conns
	if conns > len(jobs) {
		conns = len(jobs)
	}
	jobchan := make(chan UIDFetchJob)
	for i := 0; i < conns; i++ {
		go StartWorker(i, acct, jobchan)
	}

	nextjobindex := 0
	quits := 0
	for {
		if quits == conns {
			// have sent quits to everyone
			break
		}

		wantjob := <-jobchan
		if wantjob.LoUID == 0 {
			if nextjobindex < len(jobs) {
				jobchan <- jobs[nextjobindex]
				nextjobindex++
			} else {
				jobchan <- UIDFetchJob{ 0, 0 }
				quits++
			}
		} else {
			panic("unknown wantjob value")
		}
	}

	timeelapsed := time.Since(timestarted)
	msecpermessage := timeelapsed.Seconds() / float64(len(uids)) * 1000
	messagespersec := float64(len(uids)) / timeelapsed.Seconds()
	log.Printf("Finished fetching %d messages in %.2fs (%.1fms per message; %.1f messages per second)\n", len(uids), timeelapsed.Seconds(), msecpermessage, messagespersec)

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

func StartWorker(workerid int, acct *IMAPAccount, jobchan chan UIDFetchJob) {
	c := Dial(acct.Server)
	login(c, acct.Username, acct.Password)
	c.Select(mbox, true)

	for {
		jobchan <- UIDFetchJob{ 0, 0 }
		job := <-jobchan
		if job.LoUID != 0 {
			FetchMessages(c, job.LoUID, job.HiUID)
			log.Printf("Worker %d finished one job\n", workerid)
		} else {
			break
		}
	}
}

func FetchMessages(c *imap.Client, loUID, hiUID uint32) (err error) {
	set, _ := imap.NewSeqSet(fmt.Sprintf("%d:%d", loUID, hiUID))
	cmd, err := c.UIDFetch(set, "RFC822")

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
