package commands

import (
	"context"
	"fmt"

	"serverbot/internal/metrics"
)

// NewStatsHandler builds the handler that gathers and renders server metrics.
func NewStatsHandler(collector *metrics.Collector) Handler {
	return func(ctx *Context) error {
		sent, err := ctx.ReplyMessage("Recopilando metricas...")
		if err != nil {
			return err
		}

		timeout := ctx.AppConfig.CommandTimeout + 2*collector.SampleInterval()
		gatherCtx, cancel := context.WithTimeout(ctx.RequestContext, timeout)
		defer cancel()

		stats, collectErr := collector.Collect(gatherCtx)
		if collectErr != nil {
			if err := ctx.EditHTML(sent.MessageID, fmt.Sprintf("<b>Error al obtener metricas:</b>\n%s", collectErr.Error())); err != nil {
				return err
			}
			return nil
		}

		if gatherCtx.Err() == context.DeadlineExceeded {
			stats.Warnings = append(stats.Warnings, "La recopilacion excedio el tiempo limite configurado.")
		}

		report := metrics.FormatHTML(stats)
		if err := ctx.EditHTML(sent.MessageID, report); err != nil {
			return err
		}

		return nil
	}
}
