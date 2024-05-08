package harness

import (
	"os/exec"
)

type PostgresServer struct {
	ConnStr string
}

func runCommandWithLogs(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = makeLogStreamer("postgres", "stderr")
	cmd.Stdout = makeLogStreamer("postgres", "stdout")
	err := cmd.Start()
	if err != nil {
		panic("Error starting up postgres: " + err.Error())
	}
}

func StartPostgresServer() *PostgresServer {
	runCommandWithLogs(
		"pg_ctlcluster",
		"11",
		"main",
		"start",
	)

	waitTCP("127.0.0.1:5432")

	runCommandWithLogs(
		"runuser",
		"-u",
		"postgres",
		"--",
		"psql",
		"-c",
		"ALTER USER postgres WITH PASSWORD 'postgres';",
	)

	return &PostgresServer{
		ConnStr: "postgres://postgres:postgres@localhost/postgres",
	}
}

func (server *PostgresServer) Stop() {
	runCommandWithLogs(
		"pg_ctlcluster",
		"11",
		"main",
		"stop",
	)
}
