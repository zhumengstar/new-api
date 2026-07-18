package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestHidePrivateContactsForNonRoot(t *testing.T) {
	users := []*model.User{{WeChatContact: "root-contact", QQContact: "123456"}}

	hidePrivateContactsForNonRoot(common.RoleAdminUser, users)
	assert.Empty(t, users[0].WeChatContact)
	assert.Empty(t, users[0].QQContact)

	users[0].WeChatContact = "root-contact"
	users[0].QQContact = "123456"
	hidePrivateContactsForNonRoot(common.RoleRootUser, users)
	assert.Equal(t, "root-contact", users[0].WeChatContact)
	assert.Equal(t, "123456", users[0].QQContact)
}
