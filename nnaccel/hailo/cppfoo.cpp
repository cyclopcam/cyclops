#include <vector>
#include <malloc.h>

struct BufferList {
	std::vector<void*> Buffers;

	BufferList& operator=(BufferList&& b) = default;

	~BufferList() {
		for (auto b : Buffers) {
			printf("free %d\n", (int) (size_t) b);
			//free(b);
		}
	}
};

int main(int argc, char** args) {
	BufferList outer;
	{
		BufferList inner;
		inner.Buffers.push_back((void*) 123);
		printf("copying\n");
		outer = std::move(inner);
		printf("leaving\n");
	}
	printf("exit\n");
	return 0;
}