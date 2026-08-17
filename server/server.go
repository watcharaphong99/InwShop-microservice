package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/watcharaphong99/InwzaShop/config"
	"github.com/watcharaphong99/InwzaShop/modules/middlerware/middlewareHandler"
	"github.com/watcharaphong99/InwzaShop/modules/middlerware/middlewareRepository"
	middlewareusecase "github.com/watcharaphong99/InwzaShop/modules/middlerware/middlewareUsecase"
	"go.mongodb.org/mongo-driver/mongo"
)

type (
	server struct {
		app         *echo.Echo
		db          *mongo.Client
		cfg         *config.Config
		middlerware middlewareHandler.MiddlewareHandlerService
	}
)

func newMiddleware(cfg *config.Config) middlewareHandler.MiddlewareHandlerService {

	repo := middlewareRepository.NewMiddlewarerepository()
	useCase := middlewareusecase.NewMiddlewareUsecase(repo)
	return middlewareHandler.NewMiddlewareHandler(cfg, useCase)

}

func (s *server) gracefulShutdown(pctx context.Context, quit <-chan os.Signal) {
	log.Printf("Start service: %s", s.cfg.App.Name)

	<-quit
	log.Printf("Shutting down service: %s", s.cfg.App.Name)

	ctx, cancel := context.WithTimeout(pctx, 10*time.Second)
	defer cancel()

	if err := s.app.Shutdown(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func (s *server) httpListening() {
	if err := s.app.Start(s.cfg.App.Url); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error: %v", err)
	}
}

func Start(pctx context.Context, cfg *config.Config, db *mongo.Client) {
	s := &server{
		app:         echo.New(),
		db:          db,
		cfg:         cfg,
		middlerware: newMiddleware(cfg),
	}

	//Basic MiddleWare
	//Request TimeOut

	s.app.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Skipper: middleware.DefaultSkipper,
		Timeout: 30 * time.Second,
		ErrorHandler: func(err error, c echo.Context) error {
			if errors.Is(err, context.DeadlineExceeded) {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Error: Request Timeout")
			}
			return err
		},
	}))

	// CORS
	s.app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.PATCH, echo.DELETE},
	}))

	// Body Limit
	s.app.Use(middleware.BodyLimit("10M"))

	switch s.cfg.App.Name {
	case "auth":
		s.authService()
	case "player":
		s.playerService()
	case "item":
		s.itemService()
	case "inventory":
		s.inventoryService()
	case "payment":
		s.paymentService()
	}

	//GraceFul Shutdown

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	s.app.Use(middleware.RequestLogger())

	go s.gracefulShutdown(pctx, quit)

	// Listening
	s.httpListening()
}
