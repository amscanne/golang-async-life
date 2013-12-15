package main

import (
    "math/rand"
    "time"
    "github.com/nsf/termbox-go"
)

type Cell struct {
    x int
    y int
    neighbours []chan *Message
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
    next chan *Message
}

type State struct {
    gen int
    x int
    y int
    alive bool
}

func (cell *Cell) run(out chan State) {

    gen := 0
    inbox := cell.first_inbox
    var next_inbox chan *Message
    outboxes := cell.neighbours
    alive := true

    if rand.Int()%2 == 0 {
        alive = true
    } else {
        alive = false
    }

    for {
        // Make our next inbox.
        next_inbox = make(chan *Message, len(cell.neighbours))
        var msg Message
        msg.alive = alive
        msg.next = next_inbox

        // Send to all outboxes.
        for _, outbox := range outboxes {
            outbox <- &msg
        }

        // Update our internal state.
        count := 0
        outboxes = make([]chan *Message, 0, 0)
        for i := 0; i < len(cell.neighbours); i += 1 {
            msg := <- inbox

            if msg.alive {
                count += 1
            }
            outboxes = append(outboxes, msg.next)
        }

        // Send state.
        out <- State{gen, cell.x, cell.y, msg.alive}
        gen += 1

        // Figure out if we're alive.
        if count < 2 {
            alive = false
        } else if count == 2 && alive {
            alive = true
        } else if count == 3 {
            alive = true
        } else {
            alive = false
        }

        // Use next inbox.
        inbox = next_inbox
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

    for {
        select {
            case state := <-out:
                gen_color := colors[state.gen % len(colors)]
                if state.alive {
                    termbox.SetCell(state.y, state.x, rune(0x2610), termbox.ColorBlack, termbox.ColorWhite)
                } else {
                    termbox.SetCell(state.y, state.x, rune(0x0020), termbox.ColorBlack, termbox.ColorWhite)
                }
                termbox.SetCell(state.y + Y_SIZE + 1, state.x, rune(0x0020), termbox.ColorDefault, gen_color)
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
