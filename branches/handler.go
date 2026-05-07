package branches

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Repo *Repository
}

// GET /branches
func (h *Handler) GetAll(c *fiber.Ctx) error {
	list, err := h.Repo.GetAll()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Gagal mengambil data cabang"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": list})
}

// GET /branches/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "ID tidak valid"})
	}
	branch, err := h.Repo.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "Cabang tidak ditemukan"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": branch})
}

// POST /branches
func (h *Handler) Create(c *fiber.Ctx) error {
	var req BranchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Format request tidak valid"})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Nama cabang wajib diisi"})
	}
	if req.RadiusMeter == 0 {
		req.RadiusMeter = 100 // default radius 100 meter
	}
	if err := h.Repo.Create(&req); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Gagal membuat cabang"})
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "message": "Cabang berhasil dibuat"})
}

// PATCH /branches/:id
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "ID tidak valid"})
	}
	var req BranchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Format request tidak valid"})
	}
	if err := h.Repo.Update(id, &req); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Gagal update cabang"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Cabang berhasil diupdate"})
}

// DELETE /branches/:id
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "ID tidak valid"})
	}
	if err := h.Repo.Delete(id); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Gagal menghapus cabang"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Cabang berhasil dihapus"})
}
