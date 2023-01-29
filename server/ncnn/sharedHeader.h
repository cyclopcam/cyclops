// This header is shared by the C definitions exported to Go, and the internal C++ code.

typedef struct _Rect {
	int X;
	int Y;
	int Width;
	int Height;
} Rect;

// Detection is an object that a neural network has found in an image
typedef struct _Detection {
	int   Class;
	float Confidence;
	Rect  Box;
} Detection;
