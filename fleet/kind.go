package fleet

// Kind is what a host reads a page with, and it is the fact everything else
// about a host's readiness follows from.
//
// Every box in the pool was the same thing for long enough that nothing here
// asked. A host was a rented Linux box running a headed Chrome under xvfb-run
// against a signed in ChatGPT profile, so "can this host read" and "does this
// host have a browser and an account pool" were the same question, and CanOCR
// asks the second one.
//
// They came apart when a machine with a card joined the pool. It reads more
// pages than anything else in the fleet, it serves its own weights on the box,
// and it has never had an account signed into it because it does not need one.
// Asked the browser question it answers no on every count: no signed in
// profile, so the ban board printed "this host can read nothing" about the best
// reader there is, and CanOCR gated the only host with a GPU on an Xvfb it will
// never start.
//
// So a host says which it is and the gate and the report branch on it. A
// browser host is short when its account pool is empty. A reader host is short
// when the model server on the box does not answer, and the row says which
// model it is serving, because that is the fact somebody deciding where to send
// a page actually needs. Neither question is a sensible thing to ask of the
// other kind.
//
// The route file already carries the declaration: a route with Reader set names
// the program at the far end and ReaderURL says where that program's own model
// server answers. This is that distinction reaching the fleet package, which
// only ever received a name, a host and a port.
type Kind string

const (
	// Browser is a box driving a headed Chrome through chatgpt.com.
	Browser Kind = "browser"
	// Reader is a box serving its own weights and reading page images against
	// them, with no browser and no account.
	Reader Kind = "reader"
)

// Or is the kind, with browser for the zero value.
//
// Empty means a caller that has not been taught about kinds yet, and every such
// caller predates readers and means a browser box. Defaulting the other way
// would quietly stop asking those boxes for the things that actually go wrong
// on them.
func (k Kind) Or() Kind {
	if k == Reader {
		return Reader
	}
	return Browser
}
