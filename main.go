// Forward from local port 8000 to remote port 9999
package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var home, _ = os.UserHomeDir()

var (
	// Example
	// - A: your local machine
	//   - local host
	// - B: a frontend service running on port 9000
	//   - target host
	// - If you want to access B frontend service by localhost:8000
	// - the settings below is what you want
	localAddress = "localhost:8000"
	// example: 10.10.10.20:9999
	targetAddress = "targetHostIP:Port"

	// example: 10.10.10.20:22
	tunnelAddress = "tunnelHostIP:Port"
	// user: change to the user in remote who trust your local
	tunnelUser = "tunnelHostUser"

	// ----------------------------------------------------

	// true : use private key
	// false: use ssh agent
	isPrivateKeyOrElseSSHAgent = true

	privateKey = home + "/.ssh/id_rsa"
	// empty    : default will read from your environment variable: SSH_AUTH_SOCK
	// not empty: put your ssh agent sock absolute path here
	//   - example: /tmp/ssh-qsedlTZTnJLS/agent.1565122
	sshAuthSock = ""
)

func main() {
	var config *ssh.ClientConfig
	if isPrivateKeyOrElseSSHAgent {
		key, err := ioutil.ReadFile(privateKey)
		if err != nil {
			log.Fatalf("Unable to read private key: %v", err)
		}
		privateKeySigner, err := ssh.ParsePrivateKey(key)
		if err != nil {
			log.Fatalf("Unable parse private key: %v", err)
		}
		config = &ssh.ClientConfig{
			User: tunnelUser,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(privateKeySigner),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	} else {
		socket := sshAuthSock
		if socket == "" {
			socket = os.Getenv("SSH_AUTH_SOCK")
		}
		fmt.Println(socket)
		con, err := net.Dial("unix", socket)
		if err != nil {
			log.Fatalf("Failed to open SSH_AUTH_SOCK: %v", err)
		}

		agentClient := agent.NewClient(con)
		config = &ssh.ClientConfig{
			User: tunnelUser,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeysCallback(agentClient.Signers),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	}

	localListener, err := net.Listen("tcp", localAddress)
	if err != nil {
		log.Fatalf("net.Listen failed: %v", err)
	}

	fmt.Println("begin to forward " + localAddress + " to " + targetAddress)
	fmt.Println("now you can access: http://" + localAddress)
	for {
		localCon, err := localListener.Accept()
		if err != nil {
			log.Fatalf("listen.Accept failed: %v", err)
		}

		clientCon, err := ssh.Dial("tcp", tunnelAddress, config)
		if err != nil {
			log.Fatalf("ssh.Dial failed: %s", err)
		}

		sshConn, err := clientCon.Dial("tcp", targetAddress)

		// Copy local.Reader to sshConn.Writer
		go func() {
			_, err = io.Copy(sshConn, localCon)
			if err != nil {
				log.Fatalf("io.Copy failed: %v", err)
			}
		}()

		// Copy sshConn.Reader to localCon.Writer
		go func() {
			_, err = io.Copy(localCon, sshConn)
			if err != nil {
				log.Fatalf("io.Copy failed: %v", err)
			}
		}()
	}
}
