package service

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
)

type replayStats struct {
	batch   bo.Hysteria2TrafficBatch
	acks    int
	loseAck bool
}

func (s *replayStats) TrafficBatch(string) (bo.Hysteria2TrafficBatch, error) {
	return s.batch, nil
}

func (s *replayStats) AcknowledgeTrafficBatch(_ string, receipt bo.Hysteria2TrafficReceipt) error {
	if receipt != s.batch.Hysteria2TrafficReceipt {
		return errors.New("acknowledged wrong receipt")
	}
	s.acks++
	if s.loseAck {
		return errors.New("ack response lost")
	}
	return nil
}

func TestTrafficSettlementRetriesLockedDatabaseAndLostAcknowledgment(t *testing.T) {
	id, _ := setupSubscriptionTestDatabase(t)
	account, err := GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	beforeRx, beforeTx := *account.Download, *account.Upload
	api := &replayStats{batch: bo.Hysteria2TrafficBatch{
		Hysteria2TrafficReceipt: bo.Hysteria2TrafficReceipt{Epoch: "0123456789abcdef0123456789abcdef", Sequence: 1},
		Users:                   map[string]bo.Hysteria2UserTraffic{fmt.Sprintf("account:%d", id): {Tx: 7, Rx: 11}, "account:1": {Tx: 3, Rx: 5}},
	}}
	// A request completed before the consumer committed anything.
	if _, err := api.TrafficBatch(""); err != nil {
		t.Fatal(err)
	}
	if api.acks != 0 {
		t.Fatal("batch acknowledged before commit")
	}

	lock, err := sql.Open("sqlite", filepath.Join(os.Getenv("HUI_DATA"), constant.SqliteDBPath))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := lock.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if err := settleTraffic(api, "", 1); err == nil || api.acks != 0 {
		t.Fatal("locked database must not acknowledge")
	}
	if _, err := lock.Exec("ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	api.loseAck = true
	if err := settleTraffic(api, "", 1); err == nil || api.acks != 1 {
		t.Fatal("expected acknowledgement failure after commit")
	}
	if err := dao.CloseSqliteDB(); err != nil {
		t.Fatal(err)
	}
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	api.loseAck = false
	if err := settleTraffic(api, "", 1); err != nil {
		t.Fatal(err)
	}
	account, err = GetAccount(id)
	if err != nil || *account.Download != beforeRx+11 || *account.Upload != beforeTx+7 {
		t.Fatal("replayed counters not applied exactly once")
	}
	api.batch.Epoch = "fedcba9876543210fedcba9876543210"
	if err := settleTraffic(api, "", 1); err != nil {
		t.Fatal(err)
	}
	account, err = GetAccount(id)
	if err != nil || *account.Download != beforeRx+22 || *account.Upload != beforeTx+14 {
		t.Fatal("new kernel generation did not establish a separate sequence")
	}
}

func TestConnectionsToKickIncludesZeroQuotaAndDeletedAccounts(t *testing.T) {
	id, _ := setupSubscriptionTestDatabase(t)
	if err := dao.UpdateAccount([]int64{id}, map[string]interface{}{"quota": 0}); err != nil {
		t.Fatal(err)
	}
	accounts, err := dao.ListAccount(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	user, err := GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	result := connectionsToKick([]string{"sysadmin", *user.Username, "removed-user"}, accounts, 1000)
	if len(result) != 2 || result[0] != *user.Username || result[1] != "removed-user" {
		t.Fatalf("unexpected reconciliation result: %v", result)
	}
}
