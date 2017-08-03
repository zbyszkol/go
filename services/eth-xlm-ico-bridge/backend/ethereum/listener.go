package ethereum

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/stellar/go/support/log"
)

func (l *Listener) Start() error {
	l.singleRun.Lock()
	defer l.singleRun.Unlock()

	l.log = l.createLogger()
	l.log.Info("EthereumListener started")

	// TODO Address validation

	for {
		l.startGethStreamingScript()
		time.Sleep(time.Second)
	}

	return nil
}

func (l *Listener) createLogger() *log.Entry {
	logger := log.New().WithField("service", "EthereumListener")
	logger.Level = log.InfoLevel
	logger.Logger.Level = log.InfoLevel
	return logger
}

func (l *Listener) startGethStreamingScript() {
	cmd := exec.Command("geth", "--exec", gethListenerScript, "attach")

	cmdStdout, err := cmd.StdoutPipe()
	if err != nil {
		l.log.WithField("err", err).Error("Error creating strout pipe from geth")
		return
	}
	if err := cmd.Start(); err != nil {
		l.log.WithField("err", err).Error("Cannot start geth")
		return
	}

	scanner := bufio.NewScanner(cmdStdout)
	for scanner.Scan() {
		transactionJSON := scanner.Text()
		transaction, err := l.decodeTransaction(transactionJSON)
		if err != nil {
			l.log.WithFields(log.F{"err": err, "transaction": transactionJSON}).Error("Invalid transaction")
			continue
		}

		// TODO: Check if tx is valid (To is ICO account, value is positive)

		l.TransactionHandler(transaction)
	}

	if err := scanner.Err(); err != nil {
		l.log.WithField("err", err).Error("Error streaming geth stdout")
		return
	}

	if err := cmd.Wait(); err != nil {
		l.log.WithField("err", err).Error("geth exited with error code")
		return
	}
}

func (l *Listener) decodeTransaction(transactionJSON string) (Transaction, error) {
	var transaction Transaction
	err := json.Unmarshal([]byte(transactionJSON), &transaction)
	return transaction, err
}
