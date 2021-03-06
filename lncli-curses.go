package main

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/jessevdk/go-flags"
	"github.com/jroimartin/gocui"
	"github.com/lightningnetwork/lnd/lnrpc"
)

type cliOpts struct {
	LncliExec       string `short:"l" long:"lnclicmd" description:"lncli executable" default:"lncli"`
	RefreshSec      int    `short:"r" long:"refresh" description:"lncli data refresh time in seconds" default:"60"`
	RPCServer       string `long:"rpcserver" description:"host:port of ln daemon"`
	WorkDir         string `long:"lnddir" description:"path to lnd's base directory"`
	TLSCertPath     string `long:"tlscertpath" description:"path to TLS certificate"`
	NoMacaroons     bool   `long:"no-macaroons" description:"disable macaroon authentication"`
	MacaroonPath    string `long:"macaroonpath" description:"path to macaroon file"`
	MacaroonTimeOut int    `long:"macaroontimeout" description:"anti-replay macaroon validity time in seconds"`
	MacaroonIP      string `long:"macaroonip" description:"if set, lock macaroon to specific IP address"`
}

type lnclicursesContext struct {
	gocui           *gocui.Gui
	activeMainView  viewType
	views           map[viewType]viewI
	globalShortcuts []*keyHandle
	theme           themeGUI
	opts            cliOpts
	logs            []*logEntry
	printer         *message.Printer
	cliMutex        *sync.Mutex
}

var context lnclicursesContext
var status lncliStatus

func manageError(err error) {
	if err == nil {
		return
	}
	logError(err.Error())
}

func setUpdateTicker() {
	ticker := time.NewTicker(time.Second * time.Duration(context.opts.RefreshSec))
	go func() {
		for range ticker.C {
			updateData()
		}
	}()
}

func updateData() {
	manageError(status.updateLocalNodeInfo(&context))
	manageError(status.updateWalletBalance(&context))
	switch context.activeMainView {
	case channelListViewt:
		manageError(status.updateChannelList(&context))
	case peerListViewt:
		manageError(status.updatePeersList(&context))
	case pendingChannelListViewt:
		manageError(status.updatePendingChannelList(&context))
	case invoiceListViewt:
		manageError(status.updateInvoiceList(&context))
	case paymentListViewt:
		manageError(status.updatePaymentList(&context))
	case walletTransactionsViewt:
		manageError(status.updateWallletTransactionsList(&context))
	}
	refreshView()
}

func main() {

	if _, err := flags.Parse(&context.opts); err != nil {
		fmt.Println(err)
		return
	}

	status.nodes = make(map[string]lnrpc.NodeInfo)

	context.printer = message.NewPrinter(language.English)
	context.activeMainView = channelListViewt
	context.views = make(map[viewType]viewI)
	context.cliMutex = &sync.Mutex{}

	initTheme()
	initGrids()

	setUpdateTicker()
	initViews()
	switchActiveView(channelListViewt)
}

func initTheme() {
	context.theme.background = gocui.ColorBlack
	context.theme.inverted = "\x1b[48;5;21m"
	context.theme.highlight = "\x1b[38;5;75m\x1b[48;5;0m"
	context.theme.error = "\x1b[1m\x1b[38;5;196m"
	context.theme.labelHeader = "\x1b[38;5;15m\x1b[48;5;0m"
	context.theme.normal = "\x1b[38;5;15m\x1b[48;5;0m"
	context.theme.bold = "\x1b[1m"
	context.theme.gridHeader = "\x1b[48;5;89m\x1b[38;5;15m"
	context.theme.gridSelected = "\x1b[48;5;33m\x1b[38;5;15m"
}

func switchActiveView(view viewType) {
	unregisterKeyHandlers(context.views[context.activeMainView].getShortCuts())
	context.activeMainView = view
	registerKeyHandlers(context.views[view].getShortCuts())
	go updateData()
}

func initGrids() {
	initChannelListGrid()
	initPeerListGrid()
	initPendingChannelListGrid()
	initPaymentListGrid()
	initInvoiceListGrid()
	initWalletTransactionListGrid()
	initLogListGrid()
}

func initChannelListGrid() {
	context.views[channelListViewt] = newchannelListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Channels", "1", '1', gocui.ModAlt, func() { switchActiveView(channelListViewt) }, true, ""})
}

func initPeerListGrid() {
	context.views[peerListViewt] = newpeerListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Peers", "2", '2', gocui.ModAlt, func() { switchActiveView(peerListViewt) }, true, ""})
}

func initPendingChannelListGrid() {
	context.views[pendingChannelListViewt] = newpendingchannelListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Pending chnls", "3", '3', gocui.ModAlt, func() { switchActiveView(pendingChannelListViewt) }, true, ""})
}

func initPaymentListGrid() {
	context.views[paymentListViewt] = newpaymentListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Payments", "4", '4', gocui.ModAlt, func() { switchActiveView(paymentListViewt) }, true, ""})
}

func initInvoiceListGrid() {
	context.views[invoiceListViewt] = newinvoiceListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Invoices", "5", '5', gocui.ModAlt, func() { switchActiveView(invoiceListViewt) }, true, ""})
}

func initWalletTransactionListGrid() {
	context.views[walletTransactionsViewt] = newwalletTransactionListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Wallet txs", "6", '6', gocui.ModAlt, func() { switchActiveView(walletTransactionsViewt) }, true, ""})
}

func initLogListGrid() {
	context.views[logViewt] = newlogListView("main", context.theme.normal, context.theme.gridHeader, context.theme.gridSelected)
	context.globalShortcuts = append(context.globalShortcuts, &keyHandle{"Logs", "7", '7', gocui.ModAlt, func() { switchActiveView(logViewt) }, true, ""})
}
