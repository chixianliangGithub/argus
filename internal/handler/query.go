package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/argus-monitoring/argus/internal/datasource"
	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type QueryHandler struct{}

func (h *QueryHandler) Query(c *gin.Context) {
	var input struct {
		DataSourceID uint   `json:"datasource_id" binding:"required"`
		Query        string `json:"query" binding:"required"`
		Start        int64  `json:"start"` // Unix timestamp
		End          int64  `json:"end"`   // Unix timestamp
		Step         int64  `json:"step"`  // Step in seconds
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dsModel model.DataSource
	if err := model.DB.First(&dsModel, input.DataSourceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data source not found"})
		return
	}

	ds, err := datasource.NewDataSource(dsModel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result *datasource.Result
	if input.Start > 0 && input.End > 0 {
		result, err = ds.QueryRange(ctx, input.Query, input.Start, input.End, input.Step)
	} else {
		result, err = ds.Query(ctx, input.Query)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
