#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <assert.h>

// Courtesy of https://stackoverflow.com/questions/12018535/get-the-width-height-of-the-video-from-h-264-nalu

struct SPSParser {
	// Output parameters
	int Width  = 0;
	int Height = 0;

	// Internal state
	const unsigned char* m_pStart       = nullptr;
	unsigned             m_nLengthBytes = 0;
	unsigned             m_nLengthBits  = 0;
	unsigned             m_nCurrentBit  = 0;

	unsigned ReadBit() {
		//assert(m_nCurrentBit <= m_nLengthBits);
		if (m_nCurrentBit > m_nLengthBits) {
			// error
			return 0;
		}

		unsigned nIndex  = m_nCurrentBit / 8;
		unsigned nOffset = (m_nCurrentBit % 8) + 1;

		m_nCurrentBit++;
		return (m_pStart[nIndex] >> (8 - nOffset)) & 0x01;
	}

	unsigned ReadBits(int n) {
		int r = 0;
		int i;
		for (i = 0; i < n; i++) {
			r |= (ReadBit() << (n - i - 1));
		}
		return r;
	}

	unsigned ReadExponentialGolombCode() {
		int r = 0;
		int i = 0;

		while ((ReadBit() == 0) && (i < 32)) {
			i++;
		}

		r = ReadBits(i);
		r += (1 << i) - 1;
		return r;
	}

	unsigned ReadSE() {
		int r = ReadExponentialGolombCode();
		if (r & 0x01) {
			r = (r + 1) / 2;
		} else {
			r = -(r / 2);
		}
		return r;
	}

	void ParseH264SPS(const unsigned char* pStart, size_t nLen) {
		m_pStart       = pStart;
		m_nLengthBytes = (unsigned) nLen;
		m_nLengthBits  = (unsigned) nLen * 8;
		m_nCurrentBit  = 0;

		m_pStart++; // skip header bytes (eg 67 or 27)

		int frame_crop_left_offset   = 0;
		int frame_crop_right_offset  = 0;
		int frame_crop_top_offset    = 0;
		int frame_crop_bottom_offset = 0;

		int profile_idc          = ReadBits(8);
		int constraint_set0_flag = ReadBit();
		int constraint_set1_flag = ReadBit();
		int constraint_set2_flag = ReadBit();
		int constraint_set3_flag = ReadBit();
		int constraint_set4_flag = ReadBit();
		int constraint_set5_flag = ReadBit();
		int reserved_zero_2bits  = ReadBits(2);
		int level_idc            = ReadBits(8);
		int seq_parameter_set_id = ReadExponentialGolombCode();

		if (profile_idc == 100 || profile_idc == 110 ||
		    profile_idc == 122 || profile_idc == 244 ||
		    profile_idc == 44 || profile_idc == 83 ||
		    profile_idc == 86 || profile_idc == 118) {
			int chroma_format_idc = ReadExponentialGolombCode();

			if (chroma_format_idc == 3) {
				int residual_colour_transform_flag = ReadBit();
			}
			int bit_depth_luma_minus8                = ReadExponentialGolombCode();
			int bit_depth_chroma_minus8              = ReadExponentialGolombCode();
			int qpprime_y_zero_transform_bypass_flag = ReadBit();
			int seq_scaling_matrix_present_flag      = ReadBit();

			if (seq_scaling_matrix_present_flag) {
				int i = 0;
				for (i = 0; i < 8; i++) {
					int seq_scaling_list_present_flag = ReadBit();
					if (seq_scaling_list_present_flag) {
						int sizeOfScalingList = (i < 6) ? 16 : 64;
						int lastScale         = 8;
						int nextScale         = 8;
						int j                 = 0;
						for (j = 0; j < sizeOfScalingList; j++) {
							if (nextScale != 0) {
								int delta_scale = ReadSE();
								nextScale       = (lastScale + delta_scale + 256) % 256;
							}
							lastScale = (nextScale == 0) ? lastScale : nextScale;
						}
					}
				}
			}
		}

		int log2_max_frame_num_minus4 = ReadExponentialGolombCode();
		int pic_order_cnt_type        = ReadExponentialGolombCode();
		if (pic_order_cnt_type == 0) {
			int log2_max_pic_order_cnt_lsb_minus4 = ReadExponentialGolombCode();
		} else if (pic_order_cnt_type == 1) {
			int delta_pic_order_always_zero_flag      = ReadBit();
			int offset_for_non_ref_pic                = ReadSE();
			int offset_for_top_to_bottom_field        = ReadSE();
			int num_ref_frames_in_pic_order_cnt_cycle = ReadExponentialGolombCode();
			int i;
			for (i = 0; i < num_ref_frames_in_pic_order_cnt_cycle; i++) {
				ReadSE();
				//sps->offset_for_ref_frame[ i ] = ReadSE();
			}
		}
		int max_num_ref_frames                   = ReadExponentialGolombCode();
		int gaps_in_frame_num_value_allowed_flag = ReadBit();
		int pic_width_in_mbs_minus1              = ReadExponentialGolombCode();
		int pic_height_in_map_units_minus1       = ReadExponentialGolombCode();
		int frame_mbs_only_flag                  = ReadBit();
		if (!frame_mbs_only_flag) {
			int mb_adaptive_frame_field_flag = ReadBit();
		}
		int direct_8x8_inference_flag = ReadBit();
		int frame_cropping_flag       = ReadBit();
		if (frame_cropping_flag) {
			frame_crop_left_offset   = ReadExponentialGolombCode();
			frame_crop_right_offset  = ReadExponentialGolombCode();
			frame_crop_top_offset    = ReadExponentialGolombCode();
			frame_crop_bottom_offset = ReadExponentialGolombCode();
		}
		int vui_parameters_present_flag = ReadBit();
		pStart++;

		//int Width  = ((pic_width_in_mbs_minus1 + 1) * 16) - frame_crop_bottom_offset * 2 - frame_crop_top_offset * 2;
		//int Height = ((2 - frame_mbs_only_flag) * (pic_height_in_map_units_minus1 + 1) * 16) - (frame_crop_right_offset * 2) - (frame_crop_left_offset * 2);
		Width  = ((pic_width_in_mbs_minus1 + 1) * 16) - frame_crop_right_offset * 2 - frame_crop_left_offset * 2;
		Height = ((2 - frame_mbs_only_flag) * (pic_height_in_map_units_minus1 + 1) * 16) - (frame_crop_bottom_offset * 2) - (frame_crop_top_offset * 2);
	}

	// Function to read bits from a buffer
	uint32_t read_bits(uint8_t* buffer, int* bit_position, int bit_count) {
		uint32_t value = 0;
		for (int i = 0; i < bit_count; i++) {
			value <<= 1;
			value |= (buffer[*bit_position >> 3] >> (7 - (*bit_position & 7))) & 1;
			(*bit_position)++;
		}
		return value;
	}

	// From Grok 2 (2024/09/05)
	// This looks wrong.
	void ParseH265SPS(const unsigned char* pStart, size_t nLen) {
		// Function to parse SPS for width and height
		uint8_t* sps_data     = (uint8_t*) pStart;
		int      sps_length   = nLen;
		int      bit_position = 16; // Skip the NAL unit header and some initial SPS data

		// Skipping some initial parameters for brevity. You might want to parse these as well.
		for (int i = 0; i < 13; i++)
			read_bits(sps_data, &bit_position, 8); // Skip profile, level etc.

		// Read seq_parameter_set_id and other parameters if needed, here we simplify:
		read_bits(sps_data, &bit_position, 4); // seq_parameter_set_id

		// Chroma format idc
		int chroma_format_idc = read_bits(sps_data, &bit_position, 2);

		if (chroma_format_idc == 3)
			read_bits(sps_data, &bit_position, 1); // separate_colour_plane_flag

		Width  = read_bits(sps_data, &bit_position, 16); // pic_width_in_luma_samples
		Height = read_bits(sps_data, &bit_position, 16); // pic_height_in_luma_samples

		// Here, you might need to adjust for cropping, which involves reading
		// conformance_window_flag and then potentially cropping parameters.
		bool conformance_window_flag = read_bits(sps_data, &bit_position, 1);
		if (conformance_window_flag) {
			read_bits(sps_data, &bit_position, 2); // conf_win_left_offset
			read_bits(sps_data, &bit_position, 2); // conf_win_right_offset
			read_bits(sps_data, &bit_position, 2); // conf_win_top_offset
			read_bits(sps_data, &bit_position, 2); // conf_win_bottom_offset
			                                       // Adjust width and height if necessary based on these values
		}
	}
};

#ifdef __cplusplus
extern "C" {
#endif

void ParseH264SPS(const void* buf, size_t len, int* width, int* height) {
	SPSParser p;
	p.ParseH264SPS((const unsigned char*) buf, len);
	*width  = p.Width;
	*height = p.Height;
}

void ParseH265SPS(const void* buf, size_t len, int* width, int* height) {
	SPSParser p;
	p.ParseH265SPS((const unsigned char*) buf, len);
	*width  = p.Width;
	*height = p.Height;
}

#ifdef __cplusplus
}
#endif
