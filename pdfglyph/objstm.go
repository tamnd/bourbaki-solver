package pdfglyph

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

// An object stream is a PDF object holding other objects, compressed together.
// It is how a file written after 2001 keeps its small dictionaries, and it is
// why the plain scan the rest of this package does finds nothing in some
// volumes: Lie chapters 7 to 9 keeps all 35 of its /Differences arrays and all
// 39 of its font dictionaries inside four of them, so the file has not one
// occurrence of the string /Differences anywhere in it.
//
// Editing one of those in place is not possible. The rewrite the rest of this
// package does works because a shorter name padded with spaces leaves the file
// the length it was, and inside an object stream that argument stops at the
// compression: the same bytes deflated by another library come out another
// size, and the first volume tried came out 99 bytes too long to fit back in
// the span it came from.
//
// So an edited object is not written back where it was. It is appended to the
// end of the file as an object of its own, with a cross reference stream after
// it saying where it now lives, which is the incremental update the format is
// built around: nothing already in the file moves, every offset already written
// still points where it did, and a reader takes the last definition of an
// object it is given. The object stream itself is left exactly as it was, and
// the copy of the object inside it is simply never read again.

// source is a run of bytes that objects are read out of and edited in: the file
// itself, or the inflated body of one object stream.
type source struct {
	buf  []byte
	objs []object
	// stream says the buffer is the inflated body of an object stream rather
	// than the file, so an edit to it has to be appended and not written back.
	stream bool
	// changed is the objects of this source that were edited.
	changed map[int]bool
}

// edited marks an object as changed.
func (s *source) edited(num int) {
	if s.changed == nil {
		s.changed = map[int]bool{}
	}
	s.changed[num] = true
}

// sources indexes the file and every object stream in it. out is the copy that
// will be edited, and the first source is that copy itself, so an edit to the
// file's own objects lands where it should with no copying back.
func sources(out []byte) ([]*source, error) {
	all := []*source{{buf: out, objs: objects(out)}}
	for _, o := range all[0].objs {
		dict := o.dict(out)
		if !objStmRe.MatchString(dict) {
			continue
		}
		if !flateRe.MatchString(dict) {
			return nil, fmt.Errorf("object %d is an object stream this cannot read: %s", o.num, clip(dict))
		}
		start, end, ok := streamSpan(out, o)
		if !ok {
			continue
		}
		body, err := inflate(out[start:end])
		if err != nil {
			return nil, fmt.Errorf("object stream %d: %w", o.num, err)
		}
		objs, err := stmObjects(dict, body)
		if err != nil {
			return nil, fmt.Errorf("object stream %d: %w", o.num, err)
		}
		all = append(all, &source{buf: body, objs: objs, stream: true})
	}
	return all, nil
}

// update is one object to append and the number it goes back under.
type update struct {
	num  int
	body []byte
}

// updates is every object a set of sources edited inside an object stream.
func updates(srcs []*source) []update {
	var out []update
	for _, s := range srcs {
		if !s.stream {
			continue
		}
		for _, o := range s.objs {
			if s.changed[o.num] {
				out = append(out, update{num: o.num, body: bytes.TrimSpace(s.buf[o.start:o.end])})
			}
		}
	}
	return out
}

// appendUpdate writes an incremental update onto the end of a file, one object
// at a time, followed by a cross reference stream that finds them.
//
// The objects keep their numbers, so every reference already written to them
// still resolves, and the cross reference stream carries /Prev, so everything
// this update does not mention is still found through the table that was there
// before.
func appendUpdate(pdf []byte, ups []update) ([]byte, error) {
	if len(ups) == 0 {
		return pdf, nil
	}
	sort.Slice(ups, func(i, j int) bool { return ups[i].num < ups[j].num })
	t, err := lastTrailer(pdf)
	if err != nil {
		return nil, err
	}
	size, err := intField(t.dict, sizeRe)
	if err != nil {
		return nil, fmt.Errorf("the trailer has no readable /Size: %w", err)
	}
	root := keyRe("Root").FindString(t.dict)
	if root == "" {
		return nil, fmt.Errorf("the trailer names no /Root")
	}

	out := append([]byte{}, pdf...)
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	at := map[int]int{}
	for _, u := range ups {
		at[u.num] = len(out)
		out = append(out, []byte(strconv.Itoa(u.num)+" 0 obj\n")...)
		out = append(out, u.body...)
		out = append(out, []byte("\nendobj\n")...)
	}

	xrefAt := len(out)
	if t.stream {
		// The cross reference stream is itself an object, and it takes the
		// first number nothing else has.
		at[size] = xrefAt
		out = append(out, xrefStream(size, size+1, root, t, at)...)
	} else {
		out = append(out, xrefTable(size, root, t, at)...)
	}
	out = append(out, []byte(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xrefAt))...)
	return out, nil
}

// sections groups object numbers into the runs of consecutive numbers that both
// kinds of cross reference are written in.
func sections(at map[int]int) [][]int {
	nums := make([]int, 0, len(at))
	for n := range at {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var out [][]int
	for _, n := range nums {
		if len(out) > 0 {
			last := out[len(out)-1]
			if last[len(last)-1]+1 == n {
				out[len(out)-1] = append(last, n)
				continue
			}
		}
		out = append(out, []int{n})
	}
	return out
}

// xrefStream is the cross reference of an update to a file that keeps its own
// in a stream, which is every file that has object streams in it.
func xrefStream(num, size int, root string, t trailer, at map[int]int) []byte {
	var index, data bytes.Buffer
	for _, run := range sections(at) {
		fmt.Fprintf(&index, "%d %d ", run[0], len(run))
		for _, n := range run {
			data.WriteByte(1)
			var off [4]byte
			binary.BigEndian.PutUint32(off[:], uint32(at[n]))
			data.Write(off[:])
			data.Write([]byte{0, 0})
		}
	}
	head := fmt.Sprintf("%d 0 obj\n<</Type/XRef/W[1 4 2]/Size %d/Index[%s]/%s/Prev %d/Length %d",
		num, size, bytes.TrimSpace(index.Bytes()), root, t.at, data.Len())
	head += carried(t.dict)

	var out bytes.Buffer
	out.WriteString(head + ">>\nstream\n")
	out.Write(data.Bytes())
	out.WriteString("\nendstream\nendobj\n")
	return out.Bytes()
}

// xrefTable is the cross reference of an update to a file that keeps its own as
// a table, which is what a file written before object streams does and what the
// French Algèbre chapitre 8 does.
func xrefTable(size int, root string, t trailer, at map[int]int) []byte {
	var out bytes.Buffer
	out.WriteString("xref\n")
	for _, run := range sections(at) {
		fmt.Fprintf(&out, "%d %d\n", run[0], len(run))
		for _, n := range run {
			fmt.Fprintf(&out, "%010d %05d n \n", at[n], 0)
		}
	}
	fmt.Fprintf(&out, "trailer\n<</Size %d/%s/Prev %d%s>>\n", size, root, t.at, carried(t.dict))
	return out.Bytes()
}

// carried is the entries of a trailer an update has to repeat, since a reader
// takes the last trailer it is given and a file that lost its /ID or its
// /Encrypt halfway through is not readable.
func carried(dict string) string {
	out := ""
	for _, k := range []string{"Info", "ID", "Encrypt"} {
		if v := keyRe(k).FindString(dict); v != "" {
			out += "/" + v
		}
	}
	return out
}

// trailer is the cross reference a file ends on: its dictionary, where it
// starts, which the update carries forward as /Prev, and whether it is a stream
// or the older table, since the update has to be written the same way.
type trailer struct {
	dict   string
	at     int
	stream bool
}

// lastTrailer finds that cross reference.
func lastTrailer(pdf []byte) (trailer, error) {
	i := bytes.LastIndex(pdf, []byte("startxref"))
	if i < 0 {
		return trailer{}, fmt.Errorf("the file has no startxref")
	}
	fields := bytes.Fields(pdf[i+len("startxref"):])
	if len(fields) == 0 {
		return trailer{}, fmt.Errorf("startxref says nothing")
	}
	at, err := strconv.Atoi(string(fields[0]))
	if err != nil || at <= 0 || at >= len(pdf) {
		return trailer{}, fmt.Errorf("startxref says %q", fields[0])
	}
	rest := pdf[at:]
	if bytes.HasPrefix(bytes.TrimLeft(rest, " \r\n\t"), []byte("xref")) {
		j := bytes.Index(rest, []byte("trailer"))
		if j < 0 {
			return trailer{}, fmt.Errorf("the cross reference table has no trailer")
		}
		k := bytes.Index(rest[j:], []byte("startxref"))
		if k < 0 {
			k = len(rest) - j
		}
		return trailer{dict: string(rest[j : j+k]), at: at}, nil
	}
	j := bytes.Index(rest, []byte("stream"))
	if j < 0 || !bytes.Contains(rest[:j], []byte("/XRef")) {
		return trailer{}, fmt.Errorf("the file ends on a cross reference this cannot read")
	}
	return trailer{dict: string(rest[:j]), at: at, stream: true}, nil
}

// stmObjects reads the header of an object stream, which is /N pairs of an
// object number and an offset, and turns it into the same index the rest of
// this package walks. An object runs to the start of the next one, and the last
// to the end of the body.
func stmObjects(dict string, body []byte) ([]object, error) {
	n, err := intField(dict, nRe)
	if err != nil {
		return nil, err
	}
	first, err := intField(dict, firstRe)
	if err != nil {
		return nil, err
	}
	if first > len(body) {
		return nil, fmt.Errorf("/First is %d and the stream is %d bytes", first, len(body))
	}
	fields := bytes.Fields(body[:first])
	if len(fields) < 2*n {
		return nil, fmt.Errorf("/N says %d objects and the header lists %d numbers", n, len(fields))
	}
	out := make([]object, 0, n)
	for i := range n {
		num, err := strconv.Atoi(string(fields[2*i]))
		if err != nil {
			return nil, err
		}
		off, err := strconv.Atoi(string(fields[2*i+1]))
		if err != nil {
			return nil, err
		}
		end := len(body)
		if i+1 < n {
			next, err := strconv.Atoi(string(fields[2*i+3]))
			if err != nil {
				return nil, err
			}
			end = first + next
		}
		if first+off > end || end > len(body) {
			return nil, fmt.Errorf("object %d runs from %d to %d in a stream of %d",
				num, first+off, end, len(body))
		}
		out = append(out, object{num: num, start: first + off, end: end})
	}
	return out, nil
}

// streamSpan is where an object's stream data starts and stops. The bytes after
// the keyword are an end of line the specification allows to be written three
// ways.
func streamSpan(pdf []byte, o object) (int, int, bool) {
	i := bytes.Index(pdf[o.start:o.end], []byte("stream"))
	if i < 0 {
		return 0, 0, false
	}
	start := o.start + i + len("stream")
	switch {
	case bytes.HasPrefix(pdf[start:], []byte("\r\n")):
		start += 2
	case bytes.HasPrefix(pdf[start:], []byte("\n")), bytes.HasPrefix(pdf[start:], []byte("\r")):
		start++
	}
	j := bytes.Index(pdf[start:o.end], []byte("endstream"))
	if j < 0 {
		return 0, 0, false
	}
	return start, start + j, true
}

// inflate reads a zlib stream, tolerating a stream that runs out early, which
// is what a file written by a tool that padded its streams gives back.
func inflate(b []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return out, nil
}

func intField(dict string, re *regexp.Regexp) (int, error) {
	m := re.FindStringSubmatch(dict)
	if m == nil {
		return 0, fmt.Errorf("no %s", re)
	}
	return strconv.Atoi(m[1])
}

// keyRe matches one key of a trailer dictionary with its value, for the keys an
// update has to copy forward unchanged.
func keyRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)` + name + `\s*(?:\d+\s+\d+\s+R|\[.*?\]|/[A-Za-z0-9]+)`)
}

func clip(s string) string {
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

var (
	objStmRe = regexp.MustCompile(`/Type\s*/ObjStm`)
	flateRe  = regexp.MustCompile(`/Filter\s*/FlateDecode`)
	nRe      = regexp.MustCompile(`/N\s+(\d+)`)
	firstRe  = regexp.MustCompile(`/First\s+(\d+)`)
	sizeRe   = regexp.MustCompile(`/Size\s+(\d+)`)
)
