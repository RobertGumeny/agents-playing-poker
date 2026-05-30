package randomagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"

	"github.com/RobertGumeny/agent-poker/internal/wire"
)

const Version = "random/0.1.0"

type Agent struct {
	rng        *rand.Rand
	messageSeq int
}

func New(rng *rand.Rand) *Agent {
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	return &Agent{rng: rng}
}

func Run(stdin io.Reader, stdout io.Writer, rng *rand.Rand) error {
	return New(rng).Run(stdin, stdout)
}

func (a *Agent) Run(stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(stdout)

	for scanner.Scan() {
		envelope, err := wire.DecodeEnvelope(scanner.Bytes())
		if err != nil {
			return err
		}

		switch envelope.Type {
		case wire.MessageTypeSessionInit:
			if err := encoder.Encode(wire.NewMessage(wire.MessageTypeSessionReady, a.nextMessageID(), envelope.ID, wire.SessionReadyPayload{Version: Version})); err != nil {
				return fmt.Errorf("encode session_ready: %w", err)
			}
		case wire.MessageTypeYourTurn:
			var payload wire.YourTurnPayload
			if err := envelope.DecodePayload(&payload); err != nil {
				return err
			}
			action, err := a.chooseAction(payload.LegalActions)
			if err != nil {
				return err
			}
			if err := encoder.Encode(wire.NewMessage(wire.MessageTypeAction, a.nextMessageID(), envelope.ID, action)); err != nil {
				return fmt.Errorf("encode action: %w", err)
			}
		case wire.MessageTypeHandStart, wire.MessageTypeHandEnd:
			continue
		case wire.MessageTypeSessionEnd:
			return nil
		default:
			return fmt.Errorf("unsupported server message type %q", envelope.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan stdin: %w", err)
	}
	return nil
}

func (a *Agent) chooseAction(actions []wire.LegalActionOption) (wire.ActionPayload, error) {
	if len(actions) == 0 {
		return wire.ActionPayload{}, fmt.Errorf("no legal actions")
	}
	choice := actions[a.rng.Intn(len(actions))]
	payload := wire.ActionPayload{Action: choice.Action}
	switch choice.Action {
	case "call":
		if choice.Amount == nil {
			return wire.ActionPayload{}, fmt.Errorf("call action missing amount")
		}
		payload.Amount = intPtr(*choice.Amount)
	case "bet", "raise":
		if choice.Min == nil || choice.Max == nil {
			return wire.ActionPayload{}, fmt.Errorf("%s action missing min/max", choice.Action)
		}
		amount := *choice.Min
		if *choice.Max > *choice.Min {
			amount += a.rng.Intn(*choice.Max - *choice.Min + 1)
		}
		payload.Amount = intPtr(amount)
	}
	return payload, nil
}

func (a *Agent) nextMessageID() string {
	a.messageSeq++
	return fmt.Sprintf("random-%d", a.messageSeq)
}

func intPtr(v int) *int {
	return &v
}
