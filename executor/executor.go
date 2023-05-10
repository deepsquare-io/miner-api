package executor

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

type Shell struct{}

func (*Shell) ExecAs(ctx context.Context, user string, cmd string) (string, error) {
	// Get the user ID
	uid, err := lookupUserID(user)
	if err != nil {
		return "", err
	}

	c := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("cd /tmp && %s", cmd))
	fmt.Printf("exec: %+v\n", c.Args)
	// Set the user ID for the command
	c.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uid,
		},
	}

	out, err := c.CombinedOutput()
	return string(out), err
}

func lookupUserID(username string) (uint32, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, err
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(uid), nil
}
