package configdb

// Arm the system (ready to be triggered if any alarm conditions are met)
func (c *ConfigDB) Arm() error {
	return c.armDisarm(true)
}

// Disarm the system (which will also switch off the alarm if it's currently triggered)
func (c *ConfigDB) Disarm() error {
	return c.armDisarm(false)
}

// Returns true if the system is currently armed
func (c *ConfigDB) IsArmed() bool {
	c.alarmLock.Lock()
	defer c.alarmLock.Unlock()
	return c.armed
}

// Trigger the alarm immediately, regardless of whether the system is armed or not
func (c *ConfigDB) Panic() {
	c.alarmLock.Lock()
	defer c.alarmLock.Unlock()
	c.triggerAlarm()
}

// If the system is armed, then trigger the alarm
// Returns true if the alarm has been triggered (either by this call, or any other earlier call)
func (c *ConfigDB) TriggerAlarmIfArmed() bool {
	c.alarmLock.Lock()
	defer c.alarmLock.Unlock()
	if !c.armed {
		return c.alarmTriggered
	}
	c.triggerAlarm()
	return true
}

// Trigger the alarm, regardless of whether it is armed or not.
// You must be holding [alarmLock] before calling this function.
func (c *ConfigDB) triggerAlarm() {
	c.alarmTriggered = true
	if err := c.DB.Exec("UPDATE alarm_state SET triggered = 1").Error; err != nil {
		c.Log.Errorf("Failed to write alarm trigger to DB: %v", err)
	}
}

// Returns true if the alarm is currently triggered
func (c *ConfigDB) IsAlarmTriggered() bool {
	c.alarmLock.Lock()
	defer c.alarmLock.Unlock()
	return c.alarmTriggered
}

func (c *ConfigDB) armDisarm(arm bool) error {
	c.alarmLock.Lock()
	defer c.alarmLock.Unlock()
	if arm {
		if err := c.DB.Exec("UPDATE alarm_state SET armed = 1").Error; err != nil {
			return err
		}
		c.armed = true
	} else {
		// Switch off alarm if it's currently triggered
		if err := c.DB.Exec("UPDATE alarm_state SET armed = 0, triggered = 0").Error; err != nil {
			return err
		}
		c.armed = false
		c.alarmTriggered = false
	}
	return nil
}

// Read from DB into ConfigDB.armed
func (c *ConfigDB) readAlarmStateFromDB() error {
	return c.DB.Raw("SELECT armed, triggered FROM alarm_state").Row().Scan(&c.armed, &c.alarmTriggered)
}
