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

	void ParseH265SPS(const unsigned char* pStart, size_t nLen) {
		m_pStart       = pStart + 2; // skip 2-byte NAL unit header
		m_nLengthBytes = (unsigned) nLen - 2;
		m_nLengthBits  = m_nLengthBytes * 8;
		m_nCurrentBit  = 0;

		// Read sps_video_parameter_set_id (u(4))
		ReadBits(4);

		// Read sps_max_sub_layers_minus1 (u(3))
		int sps_max_sub_layers_minus1 = ReadBits(3);

		// Read sps_temporal_id_nesting_flag (u(1))
		ReadBit();

		// Parse profile_tier_level
		// General profile part
		ReadBits(2);  // general_profile_space
		ReadBit();    // general_tier_flag
		ReadBits(5);  // general_profile_idc
		ReadBits(32); // general_profile_compatibility_flag
		ReadBit();    // general_progressive_source_flag
		ReadBit();    // general_interlaced_source_flag
		ReadBit();    // general_non_packed_constraint_flag
		ReadBit();    // general_frame_only_constraint_flag
		ReadBits(44); // general_reserved_zero_44bits
		ReadBits(8);  // general_level_idc

		// Sub-layer part
		bool sub_layer_profile_present_flag[8] = {0};
		bool sub_layer_level_present_flag[8]   = {0};
		for (int i = 0; i < sps_max_sub_layers_minus1; i++) {
			sub_layer_profile_present_flag[i] = ReadBit();
			sub_layer_level_present_flag[i]   = ReadBit();
		}
		for (int i = 0; i < sps_max_sub_layers_minus1; i++) {
			if (sub_layer_profile_present_flag[i]) {
				ReadBits(2);  // sub_layer_profile_space
				ReadBit();    // sub_layer_tier_flag
				ReadBits(5);  // sub_layer_profile_idc
				ReadBits(32); // sub_layer_profile_compatibility_flag
				ReadBit();    // sub_layer_progressive_source_flag
				ReadBit();    // sub_layer_interlaced_source_flag
				ReadBit();    // sub_layer_non_packed_constraint_flag
				ReadBit();    // sub_layer_frame_only_constraint_flag
				ReadBits(44); // sub_layer_reserved_zero_44bits
			}
		}
		for (int i = 0; i < sps_max_sub_layers_minus1; i++) {
			if (sub_layer_level_present_flag[i]) {
				ReadBits(8); // sub_layer_level_idc
			}
		}

		// Read sps_seq_parameter_set_id (ue(v))
		ReadExponentialGolombCode();

		// Read chroma_format_idc (ue(v))
		int chroma_format_idc = ReadExponentialGolombCode();

		// If chroma_format_idc == 3, read separate_colour_plane_flag (u(1))
		if (chroma_format_idc == 3) {
			ReadBit();
		}

		// Read pic_width_in_luma_samples (ue(v))
		int pic_width_in_luma_samples = ReadExponentialGolombCode();

		// Read pic_height_in_luma_samples (ue(v))
		int pic_height_in_luma_samples = ReadExponentialGolombCode();

		// Read conformance_window_flag (u(1))
		int conformance_window_flag = ReadBit();

		// If conformance_window_flag, read offsets
		int conf_win_left_offset   = 0;
		int conf_win_right_offset  = 0;
		int conf_win_top_offset    = 0;
		int conf_win_bottom_offset = 0;
		if (conformance_window_flag) {
			conf_win_left_offset   = ReadExponentialGolombCode();
			conf_win_right_offset  = ReadExponentialGolombCode();
			conf_win_top_offset    = ReadExponentialGolombCode();
			conf_win_bottom_offset = ReadExponentialGolombCode();
		}

		// Calculate Width and Height
		Width  = pic_width_in_luma_samples - (conf_win_left_offset + conf_win_right_offset);
		Height = pic_height_in_luma_samples - (conf_win_top_offset + conf_win_bottom_offset);
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
