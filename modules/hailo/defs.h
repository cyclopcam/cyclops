enum cyStatus {
	cySTATUS_OK                       = 0,
	cySTATUS_STUBBED                  = 1,
	cySTATUS_MODEL_NOT_LOADED         = 2,
	cySTATUS_INVALID_INPUT_DIMENSIONS = 3,
	cySTATUS_HAILO_STATUS_OFFSET      = 10000, // This must be greater than the max Hailo status code (HAILO_STATUS_COUNT), which is 85 at this moment.
};

const char* _cyhailo_status_str_own(cyStatus s);