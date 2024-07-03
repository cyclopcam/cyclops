/**
* Copyright (c) 2021-2022 Hailo Technologies Ltd. All rights reserved.
* Distributed under the LGPL license (https://www.gnu.org/licenses/old-licenses/lgpl-2.1.txt)
**/
#ifndef _EXAMPLE_YOLOV8_POSTPROCESSING_H_
#define _EXAMPLE_YOLOV8_POSTPROCESSING_H_

#pragma once
#include "common/hailo_objects.hpp"
#include "common/hailo_common.hpp"

//__BEGIN_DECLS
//void filter(HailoROIPtr roi);
//__END_DECLS

void yolov8_postprocess_1(HailoROIPtr roi);

#endif /* _EXAMPLE_YOLOV8_POSTPROCESSING_H_ */