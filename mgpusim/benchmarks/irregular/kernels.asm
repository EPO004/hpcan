	.text
	.hsa_code_object_version 2,1
	.hsa_code_object_isa 8,0,3,"AMD","AMDGPU"
	.protected	graph_step              ; -- Begin function graph_step
	.globl	graph_step
	.p2align	8
	.type	graph_step,@function
	.amdgpu_hsa_kernel graph_step
graph_step:                             ; @graph_step
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 6
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 0
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 52
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 11
		workitem_vgpr_count = 12
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx2 s[0:1], s[4:5], 0x28
	s_lshl_b32 s2, s6, 6
	s_waitcnt lgkmcnt(0)
	s_add_i32 s0, s0, s2
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_u32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz .LBB0_6
; %bb.1:
	s_load_dwordx4 s[0:3], s[4:5], 0x18
	s_load_dwordx4 s[4:7], s[4:5], 0x0
	v_mov_b32_e32 v1, 0
	v_lshlrev_b64 v[2:3], 2, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s1
	v_add_u32_e32 v4, vcc, s0, v2
	v_addc_u32_e32 v5, vcc, v5, v3, vcc
	v_mov_b32_e32 v6, s5
	v_add_u32_e32 v2, vcc, s4, v2
	v_addc_u32_e32 v3, vcc, v6, v3, vcc
	flat_load_dwordx2 v[2:3], v[2:3]
	flat_load_dword v8, v[4:5]
	s_waitcnt vmcnt(1)
	v_cmp_lt_u32_e32 vcc, v2, v3
	s_and_saveexec_b64 s[4:5], vcc
	s_cbranch_execz .LBB0_5
; %bb.2:
	v_mov_b32_e32 v5, 0
	v_mov_b32_e32 v4, v2
	v_lshlrev_b64 v[6:7], 2, v[4:5]
	v_mov_b32_e32 v4, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v4, v7, vcc
	s_mov_b64 s[6:7], 0
	s_brev_b32 s8, -4
.LBB0_3:                                ; =>This Inner Loop Header: Depth=1
	flat_load_dword v4, v[6:7]
	v_mov_b32_e32 v11, s1
	s_waitcnt vmcnt(0)
	v_lshlrev_b64 v[9:10], 2, v[4:5]
	v_add_u32_e32 v9, vcc, s0, v9
	v_addc_u32_e32 v10, vcc, v11, v10, vcc
	flat_load_dword v4, v[9:10]
	v_add_u32_e32 v2, vcc, 1, v2
	v_add_u32_e32 v6, vcc, 4, v6
	v_addc_u32_e32 v7, vcc, 0, v7, vcc
	v_cmp_ge_u32_e32 vcc, v2, v3
	s_or_b64 s[6:7], vcc, s[6:7]
	v_mov_b32_e32 v9, s8
	s_waitcnt vmcnt(0)
	v_add_u32_e32 v10, vcc, 1, v4
	v_cmp_gt_u32_e32 vcc, s8, v4
	v_cndmask_b32_e32 v4, v9, v10, vcc
	v_min_u32_e32 v8, v4, v8
	s_andn2_b64 exec, exec, s[6:7]
	s_cbranch_execnz .LBB0_3
; %bb.4:
	s_or_b64 exec, exec, s[6:7]
.LBB0_5:
	s_or_b64 exec, exec, s[4:5]
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_mov_b32_e32 v2, s3
	v_add_u32_e32 v0, vcc, s2, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	s_waitcnt vmcnt(0)
	flat_store_dword v[0:1], v8
.LBB0_6:
	s_endpgm
.Lfunc_end0:
	.size	graph_step, .Lfunc_end0-graph_step
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 296
; NumSgprs: 11
; NumVgprs: 12
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 11
; NumVGPRsForWavesPerEU: 12
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:SCRATCH_EN: 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 6
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	sssp_step               ; -- Begin function sssp_step
	.globl	sssp_step
	.p2align	8
	.type	sssp_step,@function
	.amdgpu_hsa_kernel sssp_step
sssp_step:                              ; @sssp_step
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 3
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 6
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 0
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 52
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 14
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx2 s[0:1], s[4:5], 0x28
	s_lshl_b32 s2, s6, 6
	s_waitcnt lgkmcnt(0)
	s_add_i32 s0, s0, s2
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_u32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz .LBB1_6
; %bb.1:
	s_load_dwordx8 s[8:15], s[4:5], 0x0
	v_mov_b32_e32 v1, 0
	v_lshlrev_b64 v[2:3], 2, v[0:1]
	s_load_dwordx4 s[4:7], s[4:5], 0x20
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s15
	v_add_u32_e32 v4, vcc, s14, v2
	v_addc_u32_e32 v5, vcc, v5, v3, vcc
	v_mov_b32_e32 v6, s9
	v_add_u32_e32 v2, vcc, s8, v2
	v_addc_u32_e32 v3, vcc, v6, v3, vcc
	flat_load_dwordx2 v[2:3], v[2:3]
	flat_load_dword v10, v[4:5]
	s_waitcnt vmcnt(1)
	v_cmp_lt_u32_e32 vcc, v2, v3
	s_and_saveexec_b64 s[6:7], vcc
	s_cbranch_execz .LBB1_5
; %bb.2:
	v_mov_b32_e32 v5, 0
	v_mov_b32_e32 v4, v2
	v_lshlrev_b64 v[8:9], 2, v[4:5]
	v_mov_b32_e32 v4, s13
	v_add_u32_e32 v6, vcc, s12, v8
	v_addc_u32_e32 v7, vcc, v4, v9, vcc
	v_mov_b32_e32 v4, s11
	v_add_u32_e32 v8, vcc, s10, v8
	v_addc_u32_e32 v9, vcc, v4, v9, vcc
	s_mov_b64 s[8:9], 0
	s_mov_b32 s10, 0x3ffffffe
	s_brev_b32 s11, -4
.LBB1_3:                                ; =>This Inner Loop Header: Depth=1
	flat_load_dword v4, v[8:9]
	v_mov_b32_e32 v13, s15
	s_waitcnt vmcnt(0)
	v_lshlrev_b64 v[11:12], 2, v[4:5]
	v_add_u32_e32 v11, vcc, s14, v11
	v_addc_u32_e32 v12, vcc, v13, v12, vcc
	flat_load_dword v4, v[11:12]
	flat_load_dword v11, v[6:7]
	v_add_u32_e32 v2, vcc, 1, v2
	v_add_u32_e32 v6, vcc, 4, v6
	v_addc_u32_e32 v7, vcc, 0, v7, vcc
	v_add_u32_e32 v8, vcc, 4, v8
	v_mov_b32_e32 v12, s11
	v_addc_u32_e32 v9, vcc, 0, v9, vcc
	v_cmp_ge_u32_e32 vcc, v2, v3
	s_waitcnt vmcnt(1)
	v_cmp_lt_u32_e64 s[0:1], s10, v4
	v_sub_u32_e64 v13, s[2:3], s11, v4
	s_waitcnt vmcnt(0)
	v_add_u32_e64 v4, s[2:3], v11, v4
	v_cmp_gt_u32_e64 s[2:3], v11, v13
	s_or_b64 s[0:1], s[0:1], s[2:3]
	v_cndmask_b32_e64 v4, v4, v12, s[0:1]
	s_or_b64 s[8:9], vcc, s[8:9]
	v_min_u32_e32 v10, v4, v10
	s_andn2_b64 exec, exec, s[8:9]
	s_cbranch_execnz .LBB1_3
; %bb.4:
	s_or_b64 exec, exec, s[8:9]
.LBB1_5:
	s_or_b64 exec, exec, s[6:7]
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v0, vcc, s4, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	s_waitcnt vmcnt(0)
	flat_store_dword v[0:1], v10
.LBB1_6:
	s_endpgm
.Lfunc_end1:
	.size	sssp_step, .Lfunc_end1-sssp_step
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 380
; NumSgprs: 18
; NumVgprs: 14
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 3
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 14
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:SCRATCH_EN: 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 6
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	cc_step                 ; -- Begin function cc_step
	.globl	cc_step
	.p2align	8
	.type	cc_step,@function
	.amdgpu_hsa_kernel cc_step
cc_step:                                ; @cc_step
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 6
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 0
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 52
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 10
		workitem_vgpr_count = 12
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx2 s[0:1], s[4:5], 0x28
	s_lshl_b32 s2, s6, 6
	s_waitcnt lgkmcnt(0)
	s_add_i32 s0, s0, s2
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_u32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz .LBB2_6
; %bb.1:
	s_load_dwordx4 s[0:3], s[4:5], 0x18
	s_load_dwordx4 s[4:7], s[4:5], 0x0
	v_mov_b32_e32 v1, 0
	v_lshlrev_b64 v[2:3], 2, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s1
	v_add_u32_e32 v4, vcc, s0, v2
	v_addc_u32_e32 v5, vcc, v5, v3, vcc
	v_mov_b32_e32 v6, s5
	v_add_u32_e32 v2, vcc, s4, v2
	v_addc_u32_e32 v3, vcc, v6, v3, vcc
	flat_load_dwordx2 v[2:3], v[2:3]
	flat_load_dword v8, v[4:5]
	s_waitcnt vmcnt(1)
	v_cmp_lt_u32_e32 vcc, v2, v3
	s_and_saveexec_b64 s[4:5], vcc
	s_cbranch_execz .LBB2_5
; %bb.2:
	v_mov_b32_e32 v5, 0
	v_mov_b32_e32 v4, v2
	v_lshlrev_b64 v[6:7], 2, v[4:5]
	v_mov_b32_e32 v4, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v4, v7, vcc
	s_mov_b64 s[6:7], 0
.LBB2_3:                                ; =>This Inner Loop Header: Depth=1
	flat_load_dword v4, v[6:7]
	v_mov_b32_e32 v11, s1
	s_waitcnt vmcnt(0)
	v_lshlrev_b64 v[9:10], 2, v[4:5]
	v_add_u32_e32 v9, vcc, s0, v9
	v_addc_u32_e32 v10, vcc, v11, v10, vcc
	flat_load_dword v4, v[9:10]
	v_add_u32_e32 v2, vcc, 1, v2
	v_add_u32_e32 v6, vcc, 4, v6
	v_addc_u32_e32 v7, vcc, 0, v7, vcc
	v_cmp_ge_u32_e32 vcc, v2, v3
	s_or_b64 s[6:7], vcc, s[6:7]
	s_waitcnt vmcnt(0)
	v_min_u32_e32 v8, v4, v8
	s_andn2_b64 exec, exec, s[6:7]
	s_cbranch_execnz .LBB2_3
; %bb.4:
	s_or_b64 exec, exec, s[6:7]
.LBB2_5:
	s_or_b64 exec, exec, s[4:5]
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_mov_b32_e32 v2, s3
	v_add_u32_e32 v0, vcc, s2, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	s_waitcnt vmcnt(0)
	flat_store_dword v[0:1], v8
.LBB2_6:
	s_endpgm
.Lfunc_end2:
	.size	cc_step, .Lfunc_end2-cc_step
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 276
; NumSgprs: 10
; NumVgprs: 12
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 10
; NumVGPRsForWavesPerEU: 12
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:SCRATCH_EN: 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 6
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	rank_step               ; -- Begin function rank_step
	.globl	rank_step
	.p2align	8
	.type	rank_step,@function
	.amdgpu_hsa_kernel rank_step
rank_step:                              ; @rank_step
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 3
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 6
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 0
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 56
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 20
		workitem_vgpr_count = 15
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[0:3], s[4:5], 0x28
	s_lshl_b32 s6, s6, 6
	s_waitcnt lgkmcnt(0)
	s_add_i32 s0, s0, s6
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_u32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz .LBB3_12
; %bb.1:
	s_load_dwordx8 s[8:15], s[4:5], 0x0
	s_load_dwordx4 s[4:7], s[4:5], 0x20
	s_cmp_eq_u32 s2, 0
	s_cbranch_scc1 .LBB3_6
; %bb.2:
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s0, s12
	s_mov_b32 s1, s13
	s_mov_b32 s6, s14
	s_mov_b32 s7, s15
	v_mov_b32_e32 v7, 0
	s_mov_b32 s16, s2
	s_branch .LBB3_4
.LBB3_3:                                ;   in Loop: Header=BB3_4 Depth=1
	s_add_i32 s16, s16, -1
	s_add_u32 s6, s6, 4
	s_addc_u32 s7, s7, 0
	s_add_u32 s0, s0, 4
	s_addc_u32 s1, s1, 0
	s_cmp_eq_u32 s16, 0
	s_cbranch_scc1 .LBB3_7
.LBB3_4:                                ; =>This Inner Loop Header: Depth=1
	s_load_dword s17, s[0:1], 0x0
	s_waitcnt lgkmcnt(0)
	s_cmp_lg_u32 s17, 0
	s_cbranch_scc1 .LBB3_3
; %bb.5:                                ;   in Loop: Header=BB3_4 Depth=1
	s_load_dword s17, s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	v_add_f32_e32 v7, s17, v7
	s_branch .LBB3_3
.LBB3_6:
	v_mov_b32_e32 v7, 0
.LBB3_7:
	v_mov_b32_e32 v1, 0
	v_lshlrev_b64 v[2:3], 2, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v4, s9
	v_add_u32_e32 v2, vcc, s8, v2
	v_addc_u32_e32 v3, vcc, v4, v3, vcc
	v_add_u32_e32 v4, vcc, 1, v0
	v_mov_b32_e32 v5, v1
	v_lshlrev_b64 v[4:5], 2, v[4:5]
	v_mov_b32_e32 v6, s9
	v_add_u32_e32 v4, vcc, s8, v4
	v_addc_u32_e32 v5, vcc, v6, v5, vcc
	flat_load_dword v2, v[2:3]
	flat_load_dword v9, v[4:5]
	v_mov_b32_e32 v8, v1
	s_waitcnt vmcnt(0)
	v_cmp_lt_u32_e32 vcc, v2, v9
	s_and_saveexec_b64 s[6:7], vcc
	s_cbranch_execz .LBB3_11
; %bb.8:
	v_mov_b32_e32 v4, 0
	v_mov_b32_e32 v3, v4
	v_lshlrev_b64 v[5:6], 2, v[2:3]
	v_mov_b32_e32 v3, s11
	v_add_u32_e32 v5, vcc, s10, v5
	v_addc_u32_e32 v6, vcc, v3, v6, vcc
	s_mov_b64 s[8:9], 0
	s_mov_b32 s10, 0x6f800000
	v_mov_b32_e32 v10, 0x2f800000
	v_mov_b32_e32 v8, 0
.LBB3_9:                                ; =>This Inner Loop Header: Depth=1
	flat_load_dword v3, v[5:6]
	v_mov_b32_e32 v14, s13
	s_waitcnt vmcnt(0)
	v_lshlrev_b64 v[11:12], 2, v[3:4]
	v_add_u32_e32 v13, vcc, s12, v11
	v_addc_u32_e32 v14, vcc, v14, v12, vcc
	flat_load_dword v3, v[13:14]
	v_mov_b32_e32 v13, s15
	v_add_u32_e32 v11, vcc, s14, v11
	v_addc_u32_e32 v12, vcc, v13, v12, vcc
	flat_load_dword v11, v[11:12]
	v_add_u32_e32 v2, vcc, 1, v2
	v_add_u32_e32 v5, vcc, 4, v5
	v_addc_u32_e32 v6, vcc, 0, v6, vcc
	v_cmp_ge_u32_e32 vcc, v2, v9
	s_or_b64 s[8:9], vcc, s[8:9]
	s_waitcnt vmcnt(1)
	v_cvt_f32_u32_e32 v3, v3
	v_cmp_gt_f32_e64 s[0:1], |v3|, s10
	v_cndmask_b32_e64 v12, 1.0, v10, s[0:1]
	v_mul_f32_e32 v3, v3, v12
	v_rcp_f32_e32 v3, v3
	s_waitcnt vmcnt(0)
	v_mul_f32_e32 v3, v11, v3
	v_mac_f32_e32 v8, v12, v3
	s_andn2_b64 exec, exec, s[8:9]
	s_cbranch_execnz .LBB3_9
; %bb.10:
	s_or_b64 exec, exec, s[8:9]
.LBB3_11:
	s_or_b64 exec, exec, s[6:7]
	v_cvt_f32_u32_e32 v2, s2
	s_mov_b32 s0, 0x6f800000
	v_mov_b32_e32 v3, 0x2f800000
	v_sub_f32_e64 v4, 1.0, s3
	v_cmp_gt_f32_e64 vcc, |v2|, s0
	v_cndmask_b32_e32 v3, 1.0, v3, vcc
	v_mul_f32_e32 v2, v2, v3
	v_rcp_f32_e32 v2, v2
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s4, v0
	v_mul_f32_e32 v4, v4, v2
	v_mul_f32_e32 v2, v7, v2
	v_mac_f32_e32 v8, v3, v2
	v_mul_f32_e32 v2, s3, v8
	v_mac_f32_e32 v2, v3, v4
	v_mov_b32_e32 v3, s5
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
.LBB3_12:
	s_endpgm
.Lfunc_end3:
	.size	rank_step, .Lfunc_end3-rank_step
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 564
; NumSgprs: 20
; NumVgprs: 15
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 3
; NumSGPRsForWavesPerEU: 20
; NumVGPRsForWavesPerEU: 15
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:SCRATCH_EN: 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 6
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	gather_step             ; -- Begin function gather_step
	.globl	gather_step
	.p2align	8
	.type	gather_step,@function
	.amdgpu_hsa_kernel gather_step
gather_step:                            ; @gather_step
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 6
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 0
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 32
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 9
		workitem_vgpr_count = 6
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx2 s[0:1], s[4:5], 0x18
	s_lshl_b32 s2, s6, 6
	s_waitcnt lgkmcnt(0)
	s_add_i32 s0, s0, s2
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_u32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz .LBB4_2
; %bb.1:
	s_load_dwordx4 s[0:3], s[4:5], 0x0
	s_load_dwordx2 s[4:5], s[4:5], 0x10
	v_mov_b32_e32 v1, 0
	v_lshlrev_b64 v[2:3], 2, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s1
	v_add_u32_e32 v4, vcc, s0, v2
	v_addc_u32_e32 v5, vcc, v5, v3, vcc
	flat_load_dword v4, v[4:5]
	v_mov_b32_e32 v5, v1
	v_mov_b32_e32 v1, s3
	s_mov_b32 s0, 0x24924925
	s_waitcnt vmcnt(0)
	v_lshlrev_b64 v[4:5], 2, v[4:5]
	v_add_u32_e32 v4, vcc, s2, v4
	v_addc_u32_e32 v5, vcc, v1, v5, vcc
	flat_load_dword v1, v[4:5]
	v_mul_hi_u32 v4, v0, s0
	v_sub_u32_e32 v5, vcc, v0, v4
	v_lshrrev_b32_e32 v5, 1, v5
	v_add_u32_e32 v4, vcc, v5, v4
	v_lshrrev_b32_e32 v4, 2, v4
	v_mul_lo_u32 v4, v4, 7
	v_mov_b32_e32 v5, s5
	v_sub_u32_e32 v0, vcc, v0, v4
	v_cvt_f32_u32_e32 v4, v0
	v_add_u32_e32 v0, vcc, s4, v2
	s_waitcnt vmcnt(0)
	v_mac_f32_e32 v4, 0.5, v1
	v_addc_u32_e32 v1, vcc, v5, v3, vcc
	flat_store_dword v[0:1], v4
.LBB4_2:
	s_endpgm
.Lfunc_end4:
	.size	gather_step, .Lfunc_end4-gather_step
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 204
; NumSgprs: 9
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 9
; NumVGPRsForWavesPerEU: 6
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:SCRATCH_EN: 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 6
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	produce_step            ; -- Begin function produce_step
	.globl	produce_step
	.p2align	8
	.type	produce_step,@function
	.amdgpu_hsa_kernel produce_step
produce_step:                           ; @produce_step
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 6
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 0
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 20
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 9
		workitem_vgpr_count = 5
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[0:3], s[4:5], 0x8
	s_waitcnt lgkmcnt(0)
	s_lshl_b32 s3, s6, 6
	s_add_i32 s0, s0, s3
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_u32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz .LBB5_2
; %bb.1:
	v_mul_lo_u32 v1, v0, 13
	s_mul_i32 s0, s2, 17
	s_mov_b32 s1, 0x828cbfbf
	s_movk_i32 s2, 0xfb
	v_add_u32_e32 v2, vcc, s0, v1
	v_mul_hi_u32 v3, v2, s1
	s_load_dwordx2 s[0:1], s[4:5], 0x0
	v_mov_b32_e32 v1, 0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_lshrrev_b32_e32 v3, 7, v3
	v_mul_lo_u32 v3, v3, s2
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v4, s1
	v_sub_u32_e32 v2, vcc, v2, v3
	v_cvt_f32_u32_e32 v2, v2
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v4, v1, vcc
	flat_store_dword v[0:1], v2
.LBB5_2:
	s_endpgm
.Lfunc_end5:
	.size	produce_step, .Lfunc_end5-produce_step
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 144
; NumSgprs: 9
; NumVgprs: 5
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 9
; NumVGPRsForWavesPerEU: 5
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:SCRATCH_EN: 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 6
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.ident	"Ubuntu clang version 14.0.6"
	.section	".note.GNU-stack"
	.addrsig
	.amd_amdgpu_isa "amdgcn-amd-amdhsa--gfx803"
	.amd_amdgpu_hsa_metadata
---
Version:         [ 1, 0 ]
Kernels:
  - Name:            graph_step
    SymbolName:      'graph_step@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Attrs:
      ReqdWorkGroupSize: [ 64, 1, 1 ]
    Args:
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
    CodeProps:
      KernargSegmentSize: 52
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        11
      NumVGPRs:        12
      MaxFlatWorkGroupSize: 64
  - Name:            sssp_step
    SymbolName:      'sssp_step@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Attrs:
      ReqdWorkGroupSize: [ 64, 1, 1 ]
    Args:
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
    CodeProps:
      KernargSegmentSize: 52
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        14
      MaxFlatWorkGroupSize: 64
  - Name:            cc_step
    SymbolName:      'cc_step@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Attrs:
      ReqdWorkGroupSize: [ 64, 1, 1 ]
    Args:
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
    CodeProps:
      KernargSegmentSize: 52
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        10
      NumVGPRs:        12
      MaxFlatWorkGroupSize: 64
  - Name:            rank_step
    SymbolName:      'rank_step@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Attrs:
      ReqdWorkGroupSize: [ 64, 1, 1 ]
    Args:
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        float
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
    CodeProps:
      KernargSegmentSize: 56
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        20
      NumVGPRs:        15
      MaxFlatWorkGroupSize: 64
  - Name:            gather_step
    SymbolName:      'gather_step@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Attrs:
      ReqdWorkGroupSize: [ 64, 1, 1 ]
    Args:
      - TypeName:        'uint*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
    CodeProps:
      KernargSegmentSize: 32
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        9
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 64
  - Name:            produce_step
    SymbolName:      'produce_step@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Attrs:
      ReqdWorkGroupSize: [ 64, 1, 1 ]
    Args:
      - TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        AddrSpaceQual:   Global
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
      - TypeName:        uint
        Size:            4
        Align:           4
        ValueKind:       ByValue
        AccQual:         Default
    CodeProps:
      KernargSegmentSize: 20
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        9
      NumVGPRs:        5
      MaxFlatWorkGroupSize: 64
...

	.end_amd_amdgpu_hsa_metadata
