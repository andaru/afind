package counters

// The counters packge provides a string-to-uint64 map which is safe
// to use concurrently.

const (
	chGetDepth    = 100
	chGetAllDepth = 100
)

type ctrmap map[string]uint64

type ctrInc struct {
	key   string
	value uint64
}

type ctrGet struct {
	key     string
	channel chan uint64
}

type Counters struct {
	ctr      ctrmap
	chGet    chan ctrGet
	chGetAll chan chan ctrmap
	chInc    chan ctrInc
	chQuit   chan chan error
}

// New creates a new concurrent safe counter map, with string
// keys and uint64 values.
func New() *Counters {
	return &Counters{
		ctr:      make(ctrmap),
		chGet:    make(chan ctrGet, chGetDepth),
		chGetAll: make(chan chan ctrmap, chGetAllDepth),
		chInc:    make(chan ctrInc),
		chQuit:   make(chan chan error),
	}
}

// Start starts a Counters instance.
// As it returns itself, it can be started immediately after creation
// if desired, like so:
//   ctrs := counters.New().Start()
func (c *Counters) Start() *Counters {
	go c.loop()
	return c
}

// Get returns an individual counter value.
// Non existant keys return the default value, zero.
func (c *Counters) Get(key string) uint64 {
	if key == "" {
		return 0
	}
	reply := make(chan uint64)
	c.chGet <- ctrGet{key, reply}
	return <-reply
}

// Inc increments a key by amount.
// New keys have an initial default value of zero.
func (c *Counters) Inc(key string, amount uint64) {
	if key != "" && amount > 0 {
		request := ctrInc{key: key, value: amount}
		c.chInc <- request
	}
}

// GetAll returns the entire counter map
func (c *Counters) GetAll() ctrmap {
	reply := make(chan ctrmap)
	c.chGetAll <- reply
	return <-reply
}

func (c *Counters) Close() error {
	ch := make(chan error)
	go func() {
		c.chQuit <- ch
	}()
	return <-ch
}

func (c *Counters) loop() {
	for {
		// only handle closure once per iteration
		select {
		default: // allow fall through upon deadlock
		case ch := <-c.chQuit:
			c.close()
			ch <- nil
			return
		}

		select {
		default:
		case inc := <-c.chInc:
			if inc.key != "" {
				// may overflow
				c.ctr[inc.key] += inc.value
				// prioritise writes by returning here
				continue
			}
		}

		select {
		default:
		case get := <-c.chGet:
			if v, ok := c.ctr[get.key]; ok {
				get.channel <- v
			} else {
				get.channel <- 0
			}
			break
		case getall := <-c.chGetAll:
			m := make(ctrmap)
			for k, v := range c.ctr {
				m[k] = v
			}
			getall <- m
			break
		}
	}
}

func (c *Counters) close() {
	close(c.chGet)
	close(c.chGetAll)
	close(c.chInc)
}
