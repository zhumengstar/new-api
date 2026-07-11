package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestHideWeChatContactsForNonRoot(t *testing.T) {
	users := []*model.User{{WeChatContact: "root-contact"}}

	hideWeChatContactsForNonRoot(common.RoleAdminUser, users)
	assert.Empty(t, users[0].WeChatContact)

	users[0].WeChatContact = "root-contact"
	hideWeChatContactsForNonRoot(common.RoleRootUser, users)
	assert.Equal(t, "root-contact", users[0].WeChatContact)
}
