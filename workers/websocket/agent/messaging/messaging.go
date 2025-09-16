package messaging

import (
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
)

const (
	// MessageTypeHeartbeat represents a heartbeat message sent
	// by the agent.
	MessageTypeHeartbeat byte = 0x01
	// MessageTypeStatusMessage is a status message setnt by the agent
	MessageTypeStatusMessage byte = 0x03
	// Shell session messages
	MessageTypeCreateShell       byte = 0x04
	MessageTypeShellReady        byte = 0x05
	MessageTypeShellData         byte = 0x06
	MessageTypeShellResize       byte = 0x07
	MessageTypeShellExit         byte = 0x08
	MessageTypeClientShellClosed byte = 0x09
)

type AgentMessage struct {
	Type byte
	Data []byte
}

func (a AgentMessage) Marshal() []byte {
	ret := make([]byte, len(a.Data)+1)
	ret[0] = a.Type
	copy(ret[1:], a.Data)
	return ret
}

func UnmarshalAgentMessage(data []byte) (AgentMessage, error) {
	if len(data) < 1 {
		return AgentMessage{}, fmt.Errorf("message too short")
	}

	return AgentMessage{
		Type: data[0],
		Data: data[1:],
	}, nil
}

// MessageUnmarshaler defines types that can unmarshal from AgentMessage data
type MessageUnmarshaler interface {
	UnmarshalFromAgentMessage(data []byte) error
}

// Unmarshal unmarshals AgentMessage data into a specific message type using generics
func Unmarshal[T any, PT interface {
	*T
	MessageUnmarshaler
}](msg AgentMessage) (T, error) {
	var result T
	if err := PT(&result).UnmarshalFromAgentMessage(msg.Data); err != nil {
		return result, fmt.Errorf("failed to unmarshal message type %d: %w", msg.Type, err)
	}
	return result, nil
}

type CreateShellMessage struct {
	SessionID [16]byte
	Rows      uint32
	Cols      uint32
}

func (c *CreateShellMessage) ID() string {
	uuid, err := uuid.FromBytes(c.SessionID[:])
	if err != nil {
		return ""
	}

	return uuid.String()
}

func (c CreateShellMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeCreateShell,
		Data: make([]byte, 24),
	}

	copy(msg.Data, c.SessionID[:])
	binary.BigEndian.PutUint32(msg.Data[16:20], c.Rows)
	binary.BigEndian.PutUint32(msg.Data[20:24], c.Cols)
	return msg.Marshal()
}

func (c *CreateShellMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 24 {
		return fmt.Errorf("invalid CreateShellMessage data length: %d", len(data))
	}
	copy(c.SessionID[:], data[:16])
	c.Rows = binary.BigEndian.Uint32(data[16:20])
	c.Cols = binary.BigEndian.Uint32(data[20:24])
	return nil
}

type ShellReadyMessage struct {
	SessionID [16]byte
	IsError   byte
	Message   []byte
}

func (c *ShellReadyMessage) ID() string {
	uuid, err := uuid.FromBytes(c.SessionID[:])
	if err != nil {
		return ""
	}

	return uuid.String()
}

func (c ShellReadyMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeShellReady,
		Data: make([]byte, 17+len(c.Message)),
	}

	copy(msg.Data, c.SessionID[:])
	msg.Data[16] = c.IsError
	if len(c.Message) > 0 {
		copy(msg.Data[17:], c.Message)
	}
	return msg.Marshal()
}

func (c *ShellReadyMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 17 {
		return fmt.Errorf("invalid ShellReadyMessage data length: %d", len(data))
	}
	copy(c.SessionID[:], data[:16])
	c.IsError = data[16]
	if len(data) > 17 {
		c.Message = make([]byte, len(data)-17)
		copy(c.Message, data[17:])
	}
	return nil
}

type ShellExitMessage struct {
	SessionID [16]byte
}

func (c *ShellExitMessage) ID() string {
	uuid, err := uuid.FromBytes(c.SessionID[:])
	if err != nil {
		return ""
	}

	return uuid.String()
}

func (c ShellExitMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeShellExit,
		Data: make([]byte, 16),
	}

	copy(msg.Data, c.SessionID[:])
	return msg.Marshal()
}

func (c *ShellExitMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 16 {
		return fmt.Errorf("invalid ShellExitMessage data length: %d", len(data))
	}
	copy(c.SessionID[:], data[:16])
	return nil
}

type ClientShellClosedMessage struct {
	SessionID [16]byte
}

func (c *ClientShellClosedMessage) ID() string {
	uuid, err := uuid.FromBytes(c.SessionID[:])
	if err != nil {
		return ""
	}

	return uuid.String()
}

func (c ClientShellClosedMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeClientShellClosed,
		Data: make([]byte, 16),
	}

	copy(msg.Data, c.SessionID[:])
	return msg.Marshal()
}

func (c *ClientShellClosedMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 16 {
		return fmt.Errorf("invalid ClientShellClosedMessage data length: %d", len(data))
	}
	copy(c.SessionID[:], data[:16])
	return nil
}

type ShellDataMessage struct {
	SessionID [16]byte
	Data      []byte
}

func (c *ShellDataMessage) ID() string {
	uuid, err := uuid.FromBytes(c.SessionID[:])
	if err != nil {
		return ""
	}

	return uuid.String()
}

func (c ShellDataMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeShellData,
	}

	msg.Data = make([]byte, len(c.Data)+16)
	copy(msg.Data, c.SessionID[:])
	copy(msg.Data[16:], c.Data)
	return msg.Marshal()
}

func (c *ShellDataMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 16 {
		return fmt.Errorf("invalid ShellDataMessage data length: %d", len(data))
	}
	copy(c.SessionID[:], data[:16])
	c.Data = make([]byte, len(data)-16)
	copy(c.Data, data[16:])
	return nil
}

type ShellResizeMessage struct {
	SessionID [16]byte
	Rows      uint16
	Cols      uint16
}

func (c *ShellResizeMessage) ID() string {
	uuid, err := uuid.FromBytes(c.SessionID[:])
	if err != nil {
		return ""
	}

	return uuid.String()
}

func (c ShellResizeMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeShellResize,
	}

	msg.Data = make([]byte, 20)
	copy(msg.Data, c.SessionID[:])
	binary.BigEndian.PutUint16(msg.Data[16:18], c.Rows)
	binary.BigEndian.PutUint16(msg.Data[18:20], c.Cols)
	return msg.Marshal()
}

func (c *ShellResizeMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 20 {
		return fmt.Errorf("invalid ShellResizeMessage data length: %d", len(data))
	}
	copy(c.SessionID[:], data[:16])
	c.Rows = binary.BigEndian.Uint16(data[16:18])
	c.Cols = binary.BigEndian.Uint16(data[18:20])
	return nil
}

type RunnerUpdateMessage struct {
	AgentID uint64
	Payload []byte
}

func (s RunnerUpdateMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeStatusMessage,
	}

	msg.Data = make([]byte, 8+len(s.Payload))
	binary.BigEndian.PutUint64(msg.Data[0:8], s.AgentID)
	copy(msg.Data[8:], s.Payload)
	return msg.Marshal()
}

func (s *RunnerUpdateMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid MessageTypeStatusMessage data length: %d", len(data))
	}
	s.AgentID = binary.BigEndian.Uint64(data[0:8])
	s.Payload = make([]byte, len(data)-8)
	copy(s.Payload, data[8:])
	return nil
}

type RunnerHeartbetMessage struct {
	AgentID uint64
	Payload []byte
}

func (s RunnerHeartbetMessage) Marshal() []byte {
	msg := AgentMessage{
		Type: MessageTypeHeartbeat,
	}

	msg.Data = make([]byte, 8+len(s.Payload))
	binary.BigEndian.PutUint64(msg.Data[0:8], s.AgentID)
	copy(msg.Data[8:], s.Payload)
	return msg.Marshal()
}

func (s *RunnerHeartbetMessage) UnmarshalFromAgentMessage(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid MessageTypeStatusMessage data length: %d", len(data))
	}
	s.AgentID = binary.BigEndian.Uint64(data[0:8])
	s.Payload = make([]byte, len(data)-8)
	copy(s.Payload, data[8:])
	return nil
}
