package main

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"math/rand"
	"time"
)

type Cell struct {
	x           int
	y           int
	neighbours  []chan *Message
	first_inbox chan *Message
}

func NewCell(x int, y int) *Cell {
	cell := new(Cell)
	cell.x = x
	cell.y = y
	cell.neighbours = make([]chan *Message, 0, 0)
	cell.first_inbox = make(chan *Message, 8)
	return cell
}

func (cell *Cell) sendHello(neighbour *Cell) {
	cell.neighbours = append(cell.neighbours, neighbour.first_inbox)
}

type Message struct {
	alive bool
	next  chan *Message
}

type State struct {
	gen   int
	x     int
	y     int
	alive bool
}

func (cell *Cell) next(
	all_states chan bool,
	prev_state chan bool,
	neighbours int,
	inbox chan *Message,
	outboxes chan chan *Message) {

	// Create a new inbox & state channel.
	next_inbox := make(chan *Message, neighbours)
	next_outboxes := make(chan chan *Message, neighbours)
	next_state := make(chan bool, 1)
	next_next_state := make(chan bool, 1)

	// Start processing on the next inbox.
	// This will feed the channel of next outboxes.
	go cell.update(
		next_state,
		next_next_state,
		inbox,
		neighbours,
		next_outboxes)

	// Block for the state to be available.
	alive := <-prev_state
	all_states <- alive
	next_state <- alive

	// Create our message.
	var msg Message
	msg.alive = alive
	msg.next = next_inbox

	go func() {
		// Send notice of state to all neighbours.
		for i := 0; i < neighbours; i += 1 {
			<-outboxes <- &msg
		}
	}()

	// Start listening for the next generation.
	go cell.next(
		all_states,
		next_next_state,
		neighbours,
		next_inbox,
		next_outboxes)
}

func (cell *Cell) update(
	prev_state chan bool,
	next_state chan bool,
	inbox chan *Message,
	neighbours int,
	outboxes chan chan *Message) {

	got_alive := false
	alive := false

	// XXX Uncomment this to see the
	// non-uniform behaviour that a single
	// slow cell has on the network. This
	// is non-uniform because some cells
	// will *need* the information for this
	// cell and others will *not*!
	//if cell.x == 10 && cell.y == 10 {
	//	time.Sleep(3 * time.Second)
	//}

	// This is resolved only when
	// we require enough information
	// to make a decision.
	is_alive := func() bool {
		if !got_alive {
			alive = <-prev_state
			got_alive = true
		}
		return alive
	}

	// Make a decision.
	// NOTE: We should always terminate.
	// Once this has terminated, we will
	// have pushed out our next state.
	count := 0
	done := 0
	for {
		msg := <-inbox
		if msg.alive {
			count += 1
		}
		done += 1
		outboxes <- msg.next

		// Figure out if we're alive.
		if count > 3 {
			next_state <- false
			break
		} else if ((count < 2 && done == neighbours-1) ||
			(count < 1 && done == neighbours-2)) && !is_alive() {
			next_state <- false
			break
		} else if (count == 3 && done == neighbours) && !is_alive() {
			next_state <- true
			break
		} else if ((count == 2 && done >= neighbours-1) ||
			(count == 3 && done == neighbours)) && is_alive() {
			next_state <- true
			break
		} else if done == neighbours {
			next_state <- false
			break
		}
	}

	// Send the rest of the outboxes.
	for done < neighbours {
		msg := <-inbox
		done += 1
		outboxes <- msg.next
	}
}

func (cell *Cell) run(out chan State) {

	// Load out initial outboxes channel.
	outboxes := make(chan chan *Message, len(cell.neighbours))
	for _, outbox := range cell.neighbours {
		outboxes <- outbox
	}

	// Set our initial state.
	alive := true
	if rand.Int()%2 == 0 {
		alive = true
	} else {
		alive = false
	}
	all_states := make(chan bool, 1000)
	start_state := make(chan bool, 1)
	start_state <- alive

	// Start it.
	cell.next(
		all_states,
		start_state,
		len(cell.neighbours),
		cell.first_inbox,
		outboxes)

	gen := 0
	for {
		// Send state.
		alive = <-all_states
		out <- State{gen, cell.x, cell.y, alive}
		gen += 1
	}
}

const (
	X_SIZE = 30
	Y_SIZE = 60
)

func showState(out chan State) {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	colors := []termbox.Attribute{
		termbox.ColorRed,
		termbox.ColorGreen,
		termbox.ColorYellow,
		termbox.ColorBlue,
		termbox.ColorMagenta,
		termbox.ColorCyan,
	}

	exit_chan := make(chan bool)

	go func() {
		for {
			switch ev := termbox.PollEvent(); ev.Type {
			case termbox.EventKey:
				if ev.Key == termbox.KeyEsc {
					exit_chan <- true
				}
			case termbox.EventError:
				exit_chan <- true
			}
		}
	}()

	// Generation tracker.
	gens := make([]int, X_SIZE*Y_SIZE, X_SIZE*Y_SIZE)
	min_max_gen := func() (int, int) {
		min_gen := gens[0]
		max_gen := gens[0]
		for i := 1; i < len(gens); i += 1 {
			if gens[i] < min_gen {
				min_gen = gens[i]
			}
			if gens[i] > max_gen {
				max_gen = gens[i]
			}
		}
		return min_gen, max_gen
	}
	set_gen := func(x int, y int, gen int) {
		gens[x*Y_SIZE+y] = gen
	}

	count := 0

	for {
		select {
		case state := <-out:
			set_gen(state.x, state.y, state.gen)
			gen_color := colors[state.gen%len(colors)]
			if state.alive {
				termbox.SetCell(state.y, state.x, rune(0x2610), termbox.ColorBlack, termbox.ColorWhite)
			} else {
				termbox.SetCell(state.y, state.x, rune(0x0020), termbox.ColorBlack, termbox.ColorWhite)
			}

			// Quick hack to print the generation.
			if count%1000 == 0 {
				min_gen, max_gen := min_max_gen()
				termbox.SetCursor(0, X_SIZE+1)
				fmt.Printf("Min: %d\r\n", min_gen)
				fmt.Printf("Max: %d\r\n", max_gen)
			}
			count += 1

			termbox.SetCell(state.y+Y_SIZE+1, state.x, rune(0x0020), termbox.ColorDefault, gen_color)
			termbox.SetCell(state.y+Y_SIZE+1, state.x, rune(0x0020), termbox.ColorDefault, gen_color)

			termbox.Flush()

		case <-exit_chan:
			return
		}
	}
}

func main() {

	var cells [X_SIZE][Y_SIZE]*Cell

	rand.Seed(time.Now().UTC().UnixNano())

	for x := 0; x < X_SIZE; x += 1 {
		for y := 0; y < Y_SIZE; y += 1 {
			cells[x][y] = NewCell(x, y)
		}
	}

	out := make(chan State, 1000)

	for x := 0; x < X_SIZE; x += 1 {
		for y := 0; y < Y_SIZE; y += 1 {
			for xd := -1; xd <= 1; xd += 1 {
				for yd := -1; yd <= 1; yd += 1 {
					if xd == 0 && yd == 0 {
						continue
					}
					// Send message from x+xd,y+yd to x,y.
					cells[x][y].sendHello(cells[(x+xd+X_SIZE)%X_SIZE][(y+yd+Y_SIZE)%Y_SIZE])
				}
			}
			go cells[x][y].run(out)
		}
	}

	showState(out)
}
