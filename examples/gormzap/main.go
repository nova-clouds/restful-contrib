package main

import (
	"context"
	"time"

	"github.com/iot-labs-team/restful-contrib/gormzap"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	zapL, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	log := gormzap.New(zapL,
		gormzap.WithCustomFields(
			gormzap.String("service", "test"),
			func(ctx context.Context) zap.Field {
				v := ctx.Value("requestId")
				if v == nil {
					return zap.Skip()
				}
				if vv, ok := v.(string); ok {
					return zap.String("requestId", vv)
				}
				return zap.Skip()
			},
		),
		gormzap.WithConfig(logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			Colorful:                  false,
			IgnoreRecordNotFoundError: false,
			LogLevel:                  logger.Info,
		}),
	)
	// your dialector
	db, _ := gorm.Open(nil, &gorm.Config{Logger: log})
	// do your things
	_ = db
}
