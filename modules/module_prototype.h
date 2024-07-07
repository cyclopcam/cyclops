// In order to build an NN accelerator, you must expose the following functions:

typedef struct _NNModelSetup {
	int BatchSize;
} NNModelSetup;

typedef struct _NNModelInfo {
	int BatchSize;
	int NChan;
	int Width;
	int Height;
} NNModelInfo;

extern "C" {

int         nnm_load_model(const char* filename, const NNModelSetup* setup, void** model);
void        nnm_close_model(void* model);
void        nnm_model_info(void* model, NNModelInfo* info);
const char* nnm_status_str(int s);
int         nnm_run_model(void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle);
}