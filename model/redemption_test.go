package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteRedemptionsByIdsDeletesOnlySelectedRedemptions(t *testing.T) {
	truncateTables(t)

	redemptions := []Redemption{
		{UserId: 1, Key: "bulkdelete000000000000000001", Name: "selected-a", Quota: 100},
		{UserId: 1, Key: "bulkdelete000000000000000002", Name: "selected-b", Quota: 100},
		{UserId: 1, Key: "bulkdelete000000000000000003", Name: "kept", Quota: 100},
	}
	for i := range redemptions {
		require.NoError(t, DB.Create(&redemptions[i]).Error)
	}

	rows, err := DeleteRedemptionsByIds([]int{redemptions[0].Id, redemptions[1].Id})

	require.NoError(t, err)
	require.EqualValues(t, 2, rows)

	var remaining []Redemption
	require.NoError(t, DB.Order("id asc").Find(&remaining).Error)
	require.Len(t, remaining, 1)
	require.Equal(t, redemptions[2].Id, remaining[0].Id)
}

func TestDeleteRedemptionsByIdsRejectsEmptyIds(t *testing.T) {
	truncateTables(t)

	rows, err := DeleteRedemptionsByIds(nil)

	require.Error(t, err)
	require.EqualValues(t, 0, rows)
}
