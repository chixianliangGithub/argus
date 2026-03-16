package handler

import (
	"net/http"
	"strconv"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type DataSourceHandler struct{}

func (h *DataSourceHandler) Create(c *gin.Context) {
	var ds model.DataSource
	if err := c.ShouldBindJSON(&ds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Create(&ds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ds.Password = "" // Clear password
	c.JSON(http.StatusCreated, ds)
}

func (h *DataSourceHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var ds model.DataSource
	if err := model.DB.First(&ds, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data source not found"})
		return
	}

	ds.Password = "" // Clear password
	c.JSON(http.StatusOK, ds)
}

func (h *DataSourceHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var ds model.DataSource
	if err := model.DB.First(&ds, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data source not found"})
		return
	}

	if err := c.ShouldBindJSON(&ds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Save(&ds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ds.Password = "" // Clear password
	c.JSON(http.StatusOK, ds)
}

func (h *DataSourceHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := model.DB.Delete(&model.DataSource{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data source deleted"})
}

func (h *DataSourceHandler) List(c *gin.Context) {
	var dss []model.DataSource
	query := model.DB.Model(&model.DataSource{})

	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if typeVal := c.Query("type"); typeVal != "" {
		query = query.Where("type = ?", typeVal)
	}

	if err := query.Find(&dss).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for i := range dss {
		dss[i].Password = "" // Clear password
	}

	c.JSON(http.StatusOK, dss)
}
