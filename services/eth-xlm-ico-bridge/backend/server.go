package backend

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/stellar/go/services/eth-xlm-ico-bridge/backend/ethereum"
	"github.com/stellar/go/services/eth-xlm-ico-bridge/backend/stellar"
	"github.com/stellar/go/support/log"
)

func (s *Server) Start() {
	s.EthereumListener = ethereum.Listener{
		// TODO read from config
		ReceivingAddress:   "0x40395044ac3c0c57051906da938b54bd6557f212",
		TransactionHandler: s.onNewEthereumTransaction,
	}
	go s.EthereumListener.Start()

	s.StellarAccountConfigurator = stellar.AccountConfigurator{
	// TODO read from config
	}
	go s.StellarAccountConfigurator.Start()

	s.startHTTPServer()
}

func (s *Server) onNewEthereumTransaction(transaction ethereum.Transaction) {
	// TODO get Stellar public key from transaction.Input
	stellarPublicKey := "GC5ZR3SX3P4TZNZZ2UZETCL3YIAOHLDJSOHH33N3WY4WLH4Z3JMIQT7I"
	s.StellarAccountConfigurator.ConfigureAccount(stellarPublicKey, transaction.Value)
}

func (s *Server) startHTTPServer() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/get-state", s.handlerGetState)

	log.Info("Starting a server...")
	// TODO read from config
	err := http.ListenAndServe(":3000", r)
	if err != nil {
		log.WithField("err", err).Fatal("Cannot start server")
	}
}

func (s *Server) handlerGetState(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("welcome"))
}
