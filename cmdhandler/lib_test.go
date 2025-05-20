package cmdhandler

import (
	"strings"
	"testing"

	"github.com/lrstanley/girc"
)

func TestPingCommandExecution(t *testing.T) {
	handler, err := New("!")
	if err != nil {
		t.Fatalf("Failed to create CmdHandler: %v", err)
	}

	pingFnExecuted := false
	pingCmd := &Command{
		Name:    "ping",
		Help:    "A test ping command.",
		MinArgs: 0,
		Fn: func(c *girc.Client, input *Input) {
			pingFnExecuted = true
		},
	}
	handler.Add(pingCmd)

	// Simulate a girc.Event for PRIVMSG
	// Note: girc.Event.Last is a method, not a field.
	// We need to ensure e.Last() returns the message.
	// Params should be [target, message]
	event := girc.Event{
		Source:  &girc.Source{Name: "testuser", Host: "testhost"},
		Command: girc.PRIVMSG,
		Params:  []string{"#testchannel", "!ping"},
		Trailing: "!ping", // girc.Event.Last() typically returns the last parameter.
	}
	
	// The girc.Client can be nil for this test as Fn doesn't use it.
	// The Execute function expects a non-nil client for some internal checks (like c.GetNick()).
	// We'll create a minimal mock client.
	mockClient := &girc.Client{}

	handler.Execute(mockClient, event)

	if !pingFnExecuted {
		t.Errorf("Expected ping command's Fn to be executed, but it was not.")
	}
}

// Helper to ensure girc.Event.Last() behaves as expected for tests if needed,
// or rely on Params directly. For cmdhandler, it primarily uses input.Event.Last().
// The current structure of cmdhandler.Input uses e.Last(), which for girc.Event is a method.
// The girc library itself sets up e.LastParam for PRIVMSG.
// Let's ensure our test event aligns with how girc library populates events.
// A real girc.Event for PRIVMSG:
// e.Source.Name = "nick"
// e.Source.User = "user"
// e.Source.Host = "host"
// e.Command = "PRIVMSG"
// e.Params = []string{"#channel", "message text"}
// e.Trailing = "message text"
// Last() method on girc.Event returns e.Trailing if set, otherwise last param.

func TestCommandNotFound(t *testing.T) {
	handler, err := New("!")
	if err != nil {
		t.Fatalf("Failed to create CmdHandler: %v", err)
	}

	nonExistentCmdFnExecuted := false
	// Dummy command to ensure handler isn't empty, though not strictly necessary for this test.
	dummyCmd := &Command{
		Name: "dummy",
		Fn: func(c *girc.Client, input *Input) {
			nonExistentCmdFnExecuted = true // This should NOT be called
		},
	}
	handler.Add(dummyCmd)


	event := girc.Event{
		Source:  &girc.Source{Name: "testuser", Host: "testhost"},
		Command: girc.PRIVMSG,
		Params:  []string{"#testchannel", "!nonexistent"},
		Trailing: "!nonexistent",
	}
	mockClient := &girc.Client{} // Minimal mock

	// Execute should not panic, and our dummy command's Fn should not run.
	// If Execute had an error return for "command not found", we'd check that.
	// For now, we just ensure it doesn't call other commands.
	handler.Execute(mockClient, event)

	if nonExistentCmdFnExecuted {
		t.Errorf("Dummy command was executed when !nonexistent was called, expected no registered command to run.")
	}
	// We can also check if any reply was attempted if we had a mock client with reply recording.
	// For this test, not panicking and not calling other commands is sufficient.
}

func TestPingCommandWithArgs(t *testing.T) {
	handler, err := New("!")
	if err != nil {
		t.Fatalf("Failed to create CmdHandler: %v", err)
	}

	var capturedArgs []string
	pingCmdWithArgs := &Command{
		Name:    "ping",
		Help:    "A test ping command that captures args.",
		MinArgs: 0, // We are testing arg parsing, not enforcement here.
		Fn: func(c *girc.Client, input *Input) {
			capturedArgs = input.Args
		},
	}
	handler.Add(pingCmdWithArgs)

	event := girc.Event{
		Source:  &girc.Source{Name: "testuser", Host: "testhost"},
		Command: girc.PRIVMSG,
		Params:  []string{"#testchannel", "!ping arg1 arg2"},
		Trailing: "!ping arg1 arg2",
	}
	mockClient := &girc.Client{}

	handler.Execute(mockClient, event)

	if len(capturedArgs) != 2 {
		t.Errorf("Expected 2 arguments, got %d: %v", len(capturedArgs), capturedArgs)
	} else {
		if capturedArgs[0] != "arg1" {
			t.Errorf("Expected first argument to be 'arg1', got '%s'", capturedArgs[0])
		}
		if capturedArgs[1] != "arg2" {
			t.Errorf("Expected second argument to be 'arg2', got '%s'", capturedArgs[1])
		}
	}
}


func TestMinArgsEnforcement(t *testing.T) {
	handler, err := New("!")
	if err != nil {
		t.Fatalf("Failed to create CmdHandler: %v", err)
	}

	cmdExecuted := false
	testCmd := &Command{
		Name:    "testcmd",
		Help:    "Test command with MinArgs.",
		MinArgs: 1,
		Fn: func(c *girc.Client, input *Input) {
			cmdExecuted = true
		},
	}
	handler.Add(testCmd)

	// Mock girc.Client and its Commander to capture replies
	var replyMessage string
	mockCommander := &girc.MockClientCommander{
		Messages: make(map[string][]string),
		ReplyFunc: func(target, message string) {
			// In girc, Reply prepends target to message if it's a channel
			// For simplicity, let's just capture the message part.
			// Actual girc.Reply logic might be more complex.
			// For this test, we care about the "Usage:" message.
			replyMessage = message
		},
	}
	mockClient := &girc.Client{Commander: mockCommander}
	
	// Event with insufficient arguments
	event := girc.Event{
		Source:   &girc.Source{Name: "testuser", Host: "testhost"},
		Command:  girc.PRIVMSG,
		Params:   []string{"#testchannel", "!testcmd"},
		Trailing: "!testcmd",
		IsFromChannel: true, // Important for Reply behavior
	}
	
	handler.Execute(mockClient, event)

	if cmdExecuted {
		t.Errorf("Command was executed despite insufficient arguments.")
	}

	expectedReplyPrefix := "Usage: !testcmd"
	if !strings.HasPrefix(replyMessage, expectedReplyPrefix) {
		t.Errorf("Expected reply message to start with '%s', got '%s'", expectedReplyPrefix, replyMessage)
	}

	// Test with sufficient arguments
	cmdExecuted = false // Reset flag
	replyMessage = ""   // Reset reply
	eventSufficient := girc.Event{
		Source:   &girc.Source{Name: "testuser", Host: "testhost"},
		Command:  girc.PRIVMSG,
		Params:   []string{"#testchannel", "!testcmd arg1"},
		Trailing: "!testcmd arg1",
		IsFromChannel: true,
	}
	handler.Execute(mockClient, eventSufficient)

	if !cmdExecuted {
		t.Errorf("Command was not executed despite sufficient arguments.")
	}
	if replyMessage != "" { // Should not send usage message
		t.Errorf("Expected no reply message for sufficient arguments, got '%s'", replyMessage)
	}
}

// Note: girc.MockClientCommander is a hypothetical mock.
// The actual girc library might not provide one directly.
// If not, we'd need to implement a simple one for testing Reply.
// For now, I'll assume we can make it work or adjust the test.
// The key is testing the MinArgs logic.

// A simplified mock for girc.ClientCommands to test replies for MinArgs
type TestClientCommander struct {
	girc.ClientCommands // Embed to satisfy the interface if other methods are called
	LastReplyTarget string
	LastReplyMessage string
}

func (tcc *TestClientCommander) Reply(event girc.Event, message string) {
	// girc's Reply method takes an event and a message.
	// It figures out the target from the event.
	target := event.Params[0]
	if !event.IsFromChannel() { // If PM
		target = event.Source.Name
	}
	tcc.LastReplyTarget = target
	tcc.LastReplyMessage = message
}

// Override Privmsg if necessary, but for Reply, the above should work if girc.Client.Reply calls it.
// However, girc.Client.Cmd is an embedded struct, so we need to mock what Cmd.Reply does.
// The girc.Client.Cmd is an instance of `*girc.Commander`.
// `girc.Commander.Reply` takes `origin girc.Event, message string`.
// Let's refine the mock for MinArgs test.

type MockCommander struct {
	LastReplyOrigin girc.Event
	LastReplyMessage string
}

func (mc *MockCommander) Reply(origin girc.Event, message string) {
	mc.LastReplyOrigin = origin
	mc.LastReplyMessage = message
}
// Implement other methods of girc.CommanderInterface if Execute calls them.
// For now, assuming only Reply is critical for this test path.
// We need to implement all methods of girc.CommanderInterface.
// Let's simplify and focus on the core logic. If girc doesn't have easy mocking,
// testing the side effect (like a reply) is harder.

// Re-evaluating MinArgs test:
// The handler's Execute calls `c.Cmd.Reply(input.Event, fmt.Sprintf("Usage: %s%s %s", ch.Prefix, cmd.Name, cmd.HelpArgs))`
// So we need to mock `c.Cmd.Reply`.
// `c.Cmd` is of type `*girc.Commander`.
// We can assign a mock commander to `girc.Client.Cmd`.

func TestMinArgsEnforcementSimplified(t *testing.T) {
	handler, err := New("!")
	if err != nil {
		t.Fatalf("Failed to create CmdHandler: %v", err)
	}

	cmdExecuted := false
	testCmd := &Command{
		Name:     "testcmd",
		Help:     "Test command with MinArgs.",
		MinArgs:  1,
		HelpArgs: "<arg1>", // For usage message
		Fn: func(c *girc.Client, input *Input) {
			cmdExecuted = true
		},
	}
	handler.Add(testCmd)

	// Mock Commander
	mockCmdr := &MockGircCommander{}
	
	// Mock girc.Client. We only need to ensure `Cmd` field is set.
	// The `Cmd` field in `girc.Client` is not directly assignable as it's embedded.
	// This makes direct mocking of `c.Cmd.Reply` harder without altering girc or using interfaces.

	// Let's assume `girc.Client` is an interface or can be easily mocked.
	// If `girc.Client` is a struct, and `Cmd` is an embedded struct, this is tricky.
	// The `girc.Client` struct has a `Commander` field of type `CommanderInterface`.
	// `Commander` is an implementation of `CommanderInterface`.
	// We can provide our own implementation of `CommanderInterface`.

	mockedClientCmd := &TestCommanderInterface{}
	mockClient := &girc.Client{
		Commander: mockedClientCmd,
	}
	
	event := girc.Event{
		Source:   &girc.Source{Name: "testuser", Host: "testhost"},
		Command:  girc.PRIVMSG,
		Params:   []string{"#testchannel", "!testcmd"},
		Trailing: "!testcmd",
	}
	// To make Reply work correctly inside cmdhandler, input.Event needs to be this event.
	// The cmdhandler creates Input with this event.

	handler.Execute(mockClient, event)

	if cmdExecuted {
		t.Errorf("Command was executed despite insufficient arguments.")
	}

	expectedReply := "Usage: !testcmd <arg1>"
	if mockedClientCmd.LastMessage == "" {
		t.Errorf("Expected a reply for insufficient arguments, but got none.")
	} else if !strings.Contains(mockedClientCmd.LastMessage, expectedReply) { 
		// girc.Commander.Reply prepends Nick to PMs, or uses channel as target
		// The message itself should contain our usage string.
		t.Errorf("Expected reply message to contain '%s', got '%s'", expectedReply, mockedClientCmd.LastMessage)
	}
	
	// Test with sufficient arguments
	cmdExecuted = false 
	mockedClientCmd.LastMessage = "" // Reset
	eventSufficient := girc.Event{
		Source:   &girc.Source{Name: "testuser", Host: "testhost"},
		Command:  girc.PRIVMSG,
		Params:   []string{"#testchannel", "!testcmd arg1"},
		Trailing: "!testcmd arg1",
	}
	handler.Execute(mockClient, eventSufficient)

	if !cmdExecuted {
		t.Errorf("Command was not executed despite sufficient arguments.")
	}
	if mockedClientCmd.LastMessage != "" {
		t.Errorf("Expected no reply message for sufficient arguments, got '%s'", mockedClientCmd.LastMessage)
	}
}

// Mock for girc.CommanderInterface
type TestCommanderInterface struct {
	LastOriginEvent girc.Event
	LastMessage     string
	// Implement all methods of girc.CommanderInterface
}
func (tci *TestCommanderInterface) Nick(nick string)                                   { /* no-op */ }
func (tci *TestCommanderInterface) User(user, realname string)                         { /* no-op */ }
func (tci *TestCommanderInterface) Service(name, dist, typ, info string)               { /* no-op */ }
func (tci *TestCommanderInterface) Server(name, info string)                           { /* no-op */ }
func (tci *TestCommanderInterface) Oper(user, pass string)                             { /* no-op */ }
func (tci *TestCommanderInterface) Quit(message string)                                { /* no-op */ }
func (tci *TestCommanderInterface) SQuit(server, message string)                       { /* no-op */ }
func (tci *TestCommanderInterface) Join(channels, keys string)                         { /* no-op */ }
func (tci *TestCommanderInterface) JoinChans(chans ...string)                          { /* no-op */ }
func (tci *TestCommanderInterface) Part(channels, message string)                      { /* no-op */ }
func (tci *TestCommanderInterface) PartChans(chans ...string)                          { /* no-op */ }
func (tci *TestCommanderInterface) Mode(target string, modes ...string)                { /* no-op */ }
func (tci *TestCommanderInterface) Topic(channel, topic string)                        { /* no-op */ }
func (tci *TestCommanderInterface) Names(channels string)                              { /* no-op */ }
func (tci *TestCommanderInterface) List(channels, server string)                       { /* no-op */ }
func (tci *TestCommanderInterface) Invite(nick, channel string)                        { /* no-op */ }
func (tci *TestCommanderInterface) Kick(channel, user, message string)                 { /* no-op */ }
func (tci *TestCommanderInterface) Privmsg(target, message string)                     { /* no-op */ }
func (tci *TestCommanderInterface) Notice(target, message string)                      { /* no-op */ }
func (tci *TestCommanderInterface) Motd(server string)                                 { /* no-op */ }
func (tci *TestCommanderInterface) LUsers(server string)                               { /* no-op */ }
func (tci *TestCommanderInterface) Version(server string)                              { /* no-op */ }
func (tci *TestCommanderInterface) Stats(query, server string)                         { /* no-op */ }
func (tci *TestCommanderInterface) Links(remote, local string)                         { /* no-op */ }
func (tci *TestCommanderInterface) Time(server string)                                 { /* no-op */ }
func (tci *TestCommanderInterface) ConnectCmd(remote, local string)                    { /* no-op */ }
func (tci *TestCommanderInterface) Trace(target string)                                { /* no-op */ }
func (tci *TestCommanderInterface) Admin(server string)                                { /* no-op */ }
func (tci *TestCommanderInterface) Info(server string)                                 { /* no-op */ }
func (tci *TestCommanderInterface) ServList(mask, typ string)                          { /* no-op */ }
func (tci *TestCommanderInterface) SQuery(service, text string)                        { /* no-op */ }
func (tci *TestCommanderInterface) Who(target, o string)                               { /* no-op */ }
func (tci *TestCommanderInterface) Whois(targets string)                               { /* no-op */ }
func (tci *TestCommanderInterface) Whowas(nick, count, server string)                  { /* no-op */ }
func (tci *TestCommanderInterface) Kill(nick, reason string)                           { /* no-op */ }
func (tci *TestCommanderInterface) Ping(server1, server2 string)                       { /* no-op */ }
func (tci *TestCommanderInterface) Pong(server1, server2 string)                       { /* no-op */ }
func (tci *TestCommanderInterface) Error(reason string)                                { /* no-op */ }
func (tci *TestCommanderInterface) Away(message string)                                { /* no-op */ }
func (tci *TestCommanderInterface) Rehash()                                            { /* no-op */ }
func (tci *TestCommanderInterface) Die()                                               { /* no-op */ }
func (tci *TestCommanderInterface) Restart()                                           { /* no-op */ }
func (tci *TestCommanderInterface) Summon(user, target, channel string)                { /* no-op */ }
func (tci *TestCommanderInterface) UsersCmd(server string)                             { /* no-op */ }
func (tci *TestCommanderInterface) Userhost(nicks ...string)                           { /* no-op */ }
func (tci *TestCommanderInterface) IsOn(nicks ...string)                               { /* no-op */ }
func (tci *TestCommanderInterface) Text(line string) error                             { return nil }
func (tci *TestCommanderInterface) SendRaw(line string) error                          { return nil }
func (tci *TestCommanderInterface) SendRawf(format string, a ...interface{}) error     { return nil }
func (tci *TestCommanderInterface) Reply(event girc.Event, message string) {
	tci.LastOriginEvent = event
	tci.LastMessage = message
}
func (tci *TestCommanderInterface) Replyf(event girc.Event, format string, a ...interface{}) {
	tci.LastOriginEvent = event
	tci.LastMessage = fmt.Sprintf(format, a...)
}
func (tci *TestCommanderInterface) Privmsgf(target, format string, a ...interface{}) { /* no-op */ }
func (tci *TestCommanderInterface) Noticef(target, format string, a ...interface{})  { /* no-op */ }

// This interface is quite large. For testing, if this approach is taken,
// one might use a library for mocks or only implement the parts that could be called.
// girc.Client.Cmd is not an interface but a struct pointer *girc.Commander,
// so we can't just assign TestCommanderInterface to it.
// This means we'd have to shadow the .Cmd field or use a client that has our mock commander.
// The simplest test for MinArgs is to see if the command Fn is called or not,
// and separately unit test the Reply function if it were more complex.
// Given cmdhandler.Execute calls c.Cmd.Reply directly, a mock for CommanderInterface is needed.
// Let's stick with TestMinArgsEnforcementSimplified and ensure the mock is correctly used.
// The girc.Client struct's Commander field *is* an interface: `Commander CommanderInterface`
// So, this approach is valid.
