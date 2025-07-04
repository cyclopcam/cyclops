package configdb

func (c *ConfigDB) GetUserFromID(id int64) (*User, error) {
	user := User{}
	if err := c.DB.Find(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *ConfigDB) GetCameraFromID(id int64) (*Camera, error) {
	camera := Camera{}
	if err := c.DB.Find(&camera, id).Error; err != nil {
		return nil, err
	}
	return &camera, nil
}
