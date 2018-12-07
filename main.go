package main

import (
  "fmt"
	"syscall"
  "os"
  "math/rand"
  "time"
  "math/big"

	"golang.org/x/crypto/ssh/terminal"
	"github.com/tranvictor/ethutils/account"
	"github.com/tranvictor/ethutils/reader"
	"github.com/tranvictor/ethutils/monitor"
	"github.com/tranvictor/ethutils"
)

func getPassword(prompt string) string {
	fmt.Print(prompt)
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	return string(bytePassword)
}

func unlockAccount(keypath string) *account.Account {
	fmt.Printf("Using keystore: %s\n", keypath)
	pwd := getPassword("Enter passphrase: ")
	fmt.Printf("\n")
	acc, err := account.NewAccountFromKeystore(keypath, pwd)
	if err != nil {
		fmt.Printf("Unlocking keystore '%s' failed: %s. Abort!\n", keypath, err)
    os.Exit(1)
	}
	fmt.Printf("Keystore file is unlocked successfully. Continue...\n")
  return acc
}

// sell token on kyber with a random amount between [min, max] after each
// X minutes, randomly in [minperiod, maxperiod] range.
// This function assumes allowance is already set.
// [Non-blocking]
func dump(acc *account.Account, amount float64, token string, tokenSymbol string, min, max float64, minperiod, maxperiod int64) (wait chan bool) {
  wait = make(chan bool)
  go func() {
    mo := monitor.NewTxMonitor()
    r := reader.NewEthReader()
    decimal, err := r.ERC20Decimal(token)
    if err != nil {
      fmt.Printf("Getting decimal of %s failed: %s\n", token, err)
      wait <- false
      return
    }
    currentBalance, err := acc.ERC20Balance(token)
    if err != nil {
      fmt.Printf("Getting currentBalance failed: %s\n", err)
      wait <- false
      return
    }
    amountBig := ethutils.FloatToBigInt(amount, decimal)
    // if amount is greater than current balance, set amount to current balance
    if currentBalance.Cmp(amountBig) == -1 {
      amountBig = big.NewInt(0).Set(currentBalance)
    }
    targetBalance := big.NewInt(0).Sub(currentBalance, amountBig)
    fmt.Printf("Target KNC balance: %f\n", ethutils.BigToFloat(targetBalance, decimal))
    // loop to sell
    maxBig := ethutils.FloatToBigInt(max, decimal)
    for currentBalance.Cmp(targetBalance) > 0 {
      var tradeAmount *big.Int
      diff := big.NewInt(0).Sub(currentBalance, targetBalance)
      if diff.Cmp(maxBig) <= 0 {
        tradeAmount = big.NewInt(0).Set(diff)
      } else {
        tradeAmount = ethutils.FloatToBigInt(rand.Float64() * (max - min) + min, decimal)
      }
      sleepTime := (rand.Int63n(maxperiod - minperiod) + minperiod)
      fmt.Printf("Current %s balance: %f\n", tokenSymbol, ethutils.BigToFloat(currentBalance, decimal))
      fmt.Printf("Going to dump %f %s\n", ethutils.BigToFloat(tradeAmount, decimal), tokenSymbol)
      tx, broadcasted, errors := acc.CallContract(
        0,
        "0x818E6FECD516Ecc3849DAf6845e3EC868087B755",
        "swapTokenToEther",
        ethutils.HexToAddress(token),
        tradeAmount,
        big.NewInt(0),
      )
      if broadcasted {
        fmt.Printf("Broadcasted tx: %s\n", tx.Hash().Hex())
        info := mo.BlockingWait(tx.Hash().Hex())
        fmt.Printf("status of %s: %s\n", tx.Hash().Hex(), info.Status)
        fmt.Printf("Wait %d sec and continue\n", sleepTime)
      } else {
        fmt.Printf("Trying to dump failed: %s. Wait %d sec and try again\n", errors, sleepTime)
      }
      currentBalance, err = acc.ERC20Balance(token)
      if err != nil {
        fmt.Printf("Getting currentBalance failed: %s\n", err)
        wait <- false
        return
      }
      fmt.Printf("----------------------------------\n")
      time.Sleep(time.Duration(sleepTime) * time.Second)
    }
    wait <- true
  }()
  return wait
}

func main() {
  rand.Seed(time.Now().UTC().UnixNano())
  kncAcc := unlockAccount(os.Args[1])
  waitForKNC := dump(
    kncAcc,
    30, // dumping 30 knc
    "0xdd974d5c2e2928dea5f71b9825b8b646686bd200",
    "KNC",
    5, 10, // 5 - 10 KNC at a time
    10, 30, // each 10 to 30 second
  )
  <- waitForKNC
  // waxAcc := unlockAccount(os.Args[1])
  // waitForWAX := dump(
  //   waxAcc,
  //   "0x39Bb259F66E1C59d5ABEF88375979b4D20D98022",
  //   1500, 3800,
  //   3, 11,
  // )
  // gtoAcc := unlockAccount(os.Args[2])
  // waitForGTO := dump(
  //   gtoAcc,
  //   "0xC5bBaE50781Be1669306b9e001EFF57a2957b09d",
  //   2500, 10900,
  //   5, 15,
  // )
  // <- waitForWAX
  // <- waitForGTO
}
