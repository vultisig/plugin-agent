package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func NewErrorResponse(message string) ErrorResponse {
	return ErrorResponse{
		Message: message,
	}
}

type DeriveAddressResponse struct {
	Address       string `json:"address"`
	PublicKey     string `json:"public_key"`
	CurveType     string `json:"curve_type"`
	RootPublicKey string `json:"root_public_key"`
	ChainCode     string `json:"chain_code"`
}

// DeriveAddress godoc
// @Summary Derive address from public key
// @Description Derives a blockchain address from an ECDSA public key and chain code using BIP32 derivation
// @Tags Plugin Agent
// @Accept json
// @Produce json
// @Param publicKeyECDSA query string true "ECDSA public key in hex format"
// @Param hexChainCode query string true "Chain code in hex format for BIP32 derivation"
// @Param chain query string true "Chain name (e.g., ETH, BTC, BSC)"
// @Success 200 {object} DeriveAddressResponse "Successfully derived address"
// @Failure 400 {object} ErrorResponse "Invalid chain or parameters"
// @Failure 500 {object} ErrorResponse "Failed to derive address"
// @Router /address/derive [get]
func (s *Server) DeriveAddress(c echo.Context) error {
	publicKeyECDSA := c.QueryParam("publicKeyECDSA")
	hexChainCode := c.QueryParam("hexChainCode")
	chainString := c.QueryParam("chain")
	chain, err := common.FromString(chainString)
	if err != nil {
		s.logger.WithError(err).Error("failed to parse chain")
		return c.JSON(http.StatusBadRequest, NewErrorResponse("invalid chain"))
	}
	address, publicKey, isEdDSA, err := address.GetAddress(publicKeyECDSA, hexChainCode, chain)
	if err != nil {
		s.logger.WithError(err).Error("failed to derive address")
		return c.JSON(http.StatusInternalServerError, NewErrorResponse("failed to get address"))
	}

	curveType := "ecdsa"
	if isEdDSA {
		curveType = "eddsa"
	}

	return c.JSON(http.StatusOK, DeriveAddressResponse{
		Address:       address,
		PublicKey:     publicKey,
		CurveType:     curveType,
		RootPublicKey: publicKeyECDSA,
		ChainCode:     hexChainCode,
	})
}
