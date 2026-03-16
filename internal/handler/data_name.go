package handler

import (
	"net/http"
	"strconv"

	"github.com/argus-monitoring/argus/internal/model"
	"github.com/gin-gonic/gin"
)

type DataNameHandler struct{}

func (h *DataNameHandler) Create(c *gin.Context) {
	var dn model.DataName
	if err := c.ShouldBindJSON(&dn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Create(&dn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dn)
}

func (h *DataNameHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var dn model.DataName
	if err := model.DB.Preload("DataSource").First(&dn, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data name not found"})
		return
	}

	// Hide password
	dn.DataSource.Password = ""

	c.JSON(http.StatusOK, dn)
}

func (h *DataNameHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var dn model.DataName
	if err := model.DB.First(&dn, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data name not found"})
		return
	}

	if err := c.ShouldBindJSON(&dn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := model.DB.Save(&dn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dn)
}

func (h *DataNameHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := model.DB.Delete(&model.DataName{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data name deleted"})
}

func (h *DataNameHandler) List(c *gin.Context) {
	var dns []model.DataName
	query := model.DB.Preload("DataSource")

	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if dsID := c.Query("data_source_id"); dsID != "" {
		query = query.Where("data_source_id = ?", dsID)
	}

	if err := query.Find(&dns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Hide passwords
	for i := range dns {
		dns[i].DataSource.Password = ""
	}

	c.JSON(http.StatusOK, dns)
}
