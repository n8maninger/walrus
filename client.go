package walrus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"gitlab.com/NebulousLabs/Sia/types"
	"lukechampine.com/us/wallet"
)

// A Client communicates with a walrus server.
type Client struct {
	addr string
}

func (c *Client) req(method string, route string, data, resp interface{}) error {
	var body io.Reader
	if data != nil {
		js, _ := json.Marshal(data)
		body = bytes.NewReader(js)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%v%v", c.addr, route), body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()
	if r.StatusCode != 200 {
		err, _ := ioutil.ReadAll(r.Body)
		return errors.New(string(err))
	}
	if resp == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(resp)
}

func (c *Client) get(route string, r interface{}) error     { return c.req("GET", route, nil, r) }
func (c *Client) post(route string, d, r interface{}) error { return c.req("POST", route, d, r) }
func (c *Client) put(route string, d interface{}) error     { return c.req("PUT", route, d, nil) }
func (c *Client) delete(route string) error                 { return c.req("DELETE", route, nil, nil) }

// Addresses returns all addresses known to the wallet.
func (c *Client) Addresses() (addrs []types.UnlockHash, err error) {
	err = c.get("/addresses", &addrs)
	return
}

// AddressInfo returns information about a specific address, including its
// unlock conditions and the index it was derived from.
func (c *Client) AddressInfo(addr types.UnlockHash) (info wallet.SeedAddressInfo, err error) {
	err = c.get("/addresses/"+addr.String(), &info)
	return
}

// Balance returns the current wallet balance. If the limbo flag is true, the
// balance will reflect any transactions currently in Limbo.
func (c *Client) Balance(limbo bool) (bal types.Currency, err error) {
	err = c.get("/balance?limbo="+strconv.FormatBool(limbo), &bal)
	return
}

// Broadcast broadcasts the supplied transaction set to all connected peers.
func (c *Client) Broadcast(txnSet []types.Transaction) error {
	return c.post("/broadcast", txnSet, nil)
}

// BlockRewards returns the block rewards tracked by the wallet. If max < 0, all
// rewards are returned; otherwise, at most max rewards are returned. The
// rewards are ordered newest-to-oldest.
func (c *Client) BlockRewards(max int) (rewards []wallet.BlockReward, err error) {
	err = c.get("/blockrewards?max="+strconv.Itoa(max), &rewards)
	return
}

// ConsensusInfo returns the current blockchain height and consensus change ID.
// The latter is a unique ID that changes whenever blocks are added to the
// blockchain.
func (c *Client) ConsensusInfo() (info ResponseConsensus, err error) {
	err = c.get("/consensus", &info)
	return
}

// RecommendedFee returns the current recommended transaction fee in hastings
// per byte of the Sia-encoded transaction.
func (c *Client) RecommendedFee() (fee types.Currency, err error) {
	err = c.get("/fee", &fee)
	return
}

// FileContracts returns the file contracts tracked by the wallet. If max < 0,
// all contracts are returned; otherwise, at most max contracts are returned.
// The contracts are ordered newest-to-oldest.
func (c *Client) FileContracts(max int) (contracts []wallet.FileContract, err error) {
	err = c.get("/filecontracts?max="+strconv.Itoa(max), &contracts)
	return
}

// FileContractHistory returns the revision history of the specified file
// contract, which must be a contract tracked by the wallet.
func (c *Client) FileContractHistory(id types.FileContractID) (history []wallet.FileContract, err error) {
	err = c.get("/filecontracts/"+id.String(), &history)
	return
}

// LimboTransactions returns transactions that are in Limbo.
func (c *Client) LimboTransactions() (txns []wallet.LimboTransaction, err error) {
	err = c.get("/limbo", &txns)
	return
}

// AddToLimbo places a transaction in Limbo. The output will no longer be returned
// by Outputs or contribute to the wallet's balance.
//
// Manually adding transactions to Limbo is typically unnecessary. Calling Broadcast
// will move all transactions in the set to Limbo automatically.
func (c *Client) AddToLimbo(txn types.Transaction) (err error) {
	return c.put("/limbo/"+txn.ID().String(), txn)
}

// RemoveFromLimbo removes a transaction from Limbo.
//
// Manually removing transactions from Limbo is typically unnecessary. When a
// transaction appears in a valid block, it will be removed from Limbo
// automatically.
func (c *Client) RemoveFromLimbo(txid types.TransactionID) (err error) {
	return c.delete("/limbo/" + txid.String())
}

// Memo retrieves the memo for a transaction.
func (c *Client) Memo(txid types.TransactionID) (memo []byte, err error) {
	resp, err := http.Get(fmt.Sprintf("http://%v/memos/%v", c.addr, txid.String()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, errors.New(string(data))
	}
	return data, nil
}

// SetMemo adds a memo for a transaction, overwriting the previous memo if it
// exists.
//
// Memos are not stored on the blockchain. They exist only in the local wallet.
func (c *Client) SetMemo(txid types.TransactionID, memo []byte) (err error) {
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%v/memos/%v", c.addr, txid.String()), bytes.NewReader(memo))
	if err != nil {
		panic(err)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()
	if r.StatusCode != 200 {
		err, _ := ioutil.ReadAll(r.Body)
		return errors.New(string(err))
	}
	return nil
}

// SeedIndex returns the index that should be used to derive the next address.
func (c *Client) SeedIndex() (index uint64, err error) {
	err = c.get("/seedindex", &index)
	return
}

// Transactions lists the IDs of transactions relevant to the wallet. If max <
// 0, all such IDs are returned; otherwise, at most max IDs are returned. The
// IDs are ordered newest-to-oldest.
func (c *Client) Transactions(max int) (txids []types.TransactionID, err error) {
	err = c.get("/transactions?max="+strconv.Itoa(max), &txids)
	return
}

// TransactionsByAddress lists the IDs of transactions relevant to the specified
// address, which must be owned by the wallet. If max < 0, all such IDs are
// returned; otherwise, at most max IDs are returned. The IDs are ordered
// newest-to-oldest.
func (c *Client) TransactionsByAddress(addr types.UnlockHash, max int) (txids []types.TransactionID, err error) {
	err = c.get("/transactions?max="+strconv.Itoa(max)+"&addr="+addr.String(), &txids)
	return
}

// Transaction returns the transaction with the specified ID, as well as inflow,
// outflow, and fee information. The transaction must be relevant to the wallet.
func (c *Client) Transaction(txid types.TransactionID) (txn ResponseTransactionsID, err error) {
	err = c.get("/transactions/"+txid.String(), &txn)
	return
}

// UnconfirmedParents returns any parents of txn that are in Limbo. These
// transactions will need to be included in the transaction set passed to
// Broadcast.
func (c *Client) UnconfirmedParents(txn types.Transaction) (parents []wallet.LimboTransaction, err error) {
	err = c.post("/unconfirmedparents", txn, &parents)
	return
}

// UnspentOutputs returns the outputs that the wallet can spend. If the limbo
// flag is true, the outputs will reflect any transactions currently in Limbo.
func (c *Client) UnspentOutputs(limbo bool) (utxos []wallet.UnspentOutput, err error) {
	err = c.get("/utxos?limbo="+strconv.FormatBool(limbo), &utxos)
	return
}

// AddAddress adds a set of address metadata to the wallet. Future
// transactions and outputs relevant to this address will be considered relevant
// to the wallet.
//
// Importing an address does NOT import transactions and outputs relevant to
// that address that are already in the blockchain.
func (c *Client) AddAddress(info wallet.SeedAddressInfo) error {
	return c.post("/addresses", info, new(types.UnlockHash))
}

// RemoveAddress removes an address from the wallet. Future transactions and
// outputs relevant to this address will not be considered relevant to the
// wallet.
//
// Removing an address does NOT remove transactions and outputs relevant to that
// address that are already recorded in the wallet.
func (c *Client) RemoveAddress(addr types.UnlockHash) error {
	return c.delete("/addresses/" + addr.String())
}

// NewClient returns a client that communicates with a walrus server listening
// on the specified address.
func NewClient(addr string) *Client {
	// use https by default
	if !strings.HasPrefix(addr, "https://") && !strings.HasPrefix(addr, "http://") {
		addr = "https://" + addr
	}
	return &Client{addr}
}
