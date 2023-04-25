package xpanels

//
//func TestAddClient_valid(t *testing.T) {
//	xs := NewXrayService("127.0.0.1:10085")
//
//	c := XClient{
//		Uuid:    "myTestClient",
//		Email:   "myTestClient@server",
//		Level:   0,
//		AlterId: 30,
//	}
//	// Assumed an inbound with "inbound-9095-vless" tag is listening on 9095
//	err := xs.AddClient(c, "inbound-9095-vless")
//	assert.NoError(t, err)
//	// test if client already added. We expect to receive a duplicate error from xray core
//	err = xs.AddClient(c, "inbound-9095-vless")
//	assert.ErrorContains(t, err, c.Email+" already exists")
//}
//
//func TestAddClient_InvalidInbound(t *testing.T) {
//	xs := NewXrayService("127.0.0.1:10085")
//
//	c := XClient{
//		Uuid:    "myTestClient",
//		Email:   "myTestClient@server",
//		Level:   0,
//		AlterId: 30,
//	}
//	// inbound does not exists
//	err := xs.AddClient(c, "vless_randomName")
//	assert.ErrorContains(t, err, "code = Unknown")
//	err = xs.AddClient(c, "vmess_randomName")
//	assert.ErrorContains(t, err, "code = Unknown")
//	err = xs.AddClient(c, "trojan_randomName")
//	assert.ErrorContains(t, err, "code = Unknown")
//}
