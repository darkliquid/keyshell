package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// runVerbClient sends one verb to a running host over the IPC socket and
// prints any response. It is what `keyshell toggle` and friends resolve to.
func runVerbClient(verb string, args []string) int {
	fs := newFlagSet("keyshell " + verb)
	socket := fs.String("socket", "", "IPC socket path override")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 && verb != "openMode" {
		fmt.Fprintf(os.Stderr, "keyshell %s: unexpected argument %q\n", verb, fs.Arg(0))
		return 2
	}

	payload := verb
	if rest := fs.Args(); len(rest) > 0 {
		payload = verb + " " + strings.Join(rest, " ")
	}

	conn, err := dialIPC(*socket, 2*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keyshell not running:", err)
		return 1
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte(payload + "\n")); err != nil {
		fmt.Fprintln(os.Stderr, "send:", err)
		return 1
	}

	// Query-style verbs answer with a line of JSON; control verbs answer
	// with nothing. A short deadline separates the two without hanging a
	// keybind script.
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && resp == "" {
		return 0
	}
	line := strings.TrimRight(resp, "\n")
	if strings.Contains(line, "\"error\"") {
		fmt.Fprintln(os.Stderr, strings.TrimSpace(line))
		return 1
	}
	fmt.Println(line)
	return 0
}

// queryHost sends a query verb to a running host and parses the JSON answer.
func queryHost(socket, mode, text string) (queryResponse, error) {
	conn, err := dialIPC(socket, 2*time.Second)
	if err != nil {
		return queryResponse{}, err
	}
	defer func() { _ = conn.Close() }()

	verb := "query " + text
	if mode != "" && mode != "all" {
		verb = "queryMode " + mode + " " + text
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write([]byte(verb + "\n")); err != nil {
		return queryResponse{}, err
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && resp == "" {
		return queryResponse{}, err
	}
	line := strings.TrimSpace(strings.TrimRight(resp, "\n"))
	if line == "" {
		return queryResponse{}, fmt.Errorf("host closed the connection without an answer")
	}
	return decodeQueryResponse(line)
}
