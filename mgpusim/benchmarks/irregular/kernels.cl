// Fixed 64-workitem groups avoid an external OpenCL runtime library.
// All graph updates use previous-iteration buffers and unique destination writers.
#define ID (start + __builtin_amdgcn_workgroup_id_x()*64u + __builtin_amdgcn_workitem_id_x())
#define INF 0x3fffffffu
__attribute__((reqd_work_group_size(64,1,1)))
__kernel void graph_step(__global const uint *row, __global const uint *col,
 __global const uint *weight, __global const uint *old, __global uint *out,
 uint start, uint end, uint unused) {
 uint v=ID;if(v>=end)return;uint best=old[v];
 for(uint j=row[v];j<row[v+1];j++){
   uint u=col[j],x=old[u];
   if(0!=2){uint w=0==0?1:weight[j];x=x<INF && w<=INF-x?x+w:INF;}
   if(x<best)best=x;
 }
 out[v]=best;
}
__attribute__((reqd_work_group_size(64,1,1)))
__kernel void sssp_step(__global const uint *row, __global const uint *col,
 __global const uint *weight, __global const uint *old, __global uint *out,
 uint start, uint end, uint unused) {
 uint v=ID;if(v>=end)return;uint best=old[v];
 for(uint j=row[v];j<row[v+1];j++){
   uint u=col[j],x=old[u];
   if(1!=2){uint w=1==0?1:weight[j];x=x<INF && w<=INF-x?x+w:INF;}
   if(x<best)best=x;
 }
 out[v]=best;
}
__attribute__((reqd_work_group_size(64,1,1)))
__kernel void cc_step(__global const uint *row, __global const uint *col,
 __global const uint *weight, __global const uint *old, __global uint *out,
 uint start, uint end, uint unused) {
 uint v=ID;if(v>=end)return;uint best=old[v];
 for(uint j=row[v];j<row[v+1];j++){
   uint u=col[j],x=old[u];
   if(2!=2){uint w=2==0?1:weight[j];x=x<INF && w<=INF-x?x+w:INF;}
   if(x<best)best=x;
 }
 out[v]=best;
}
__attribute__((reqd_work_group_size(64,1,1)))
__kernel void rank_step(__global const uint *row,__global const uint *col,
 __global const uint *degree,__global const float *old,__global float *out,
 uint start,uint end,uint n,float damping) {
 uint v=ID;if(v>=end)return;float sum=0.0f,sink=0.0f;
 for(uint u=0;u<n;u++)if(degree[u]==0)sink+=old[u];
 for(uint j=row[v];j<row[v+1];j++){uint u=col[j];sum+=old[u]/(float)degree[u];}
 out[v]=(1.0f-damping)/(float)n+damping*(sink/(float)n+sum);
}
__attribute__((reqd_work_group_size(64,1,1)))
__kernel void gather_step(__global const uint *index,__global const float *data,
 __global float *out,uint start,uint end) {
 uint i=ID;if(i<end)out[i]=data[index[i]]*0.5f+(float)(i%7u);
}
__attribute__((reqd_work_group_size(64,1,1)))
__kernel void produce_step(__global float *data,uint start,uint end,uint epoch) {
 uint i=ID;if(i<end)data[i]=(float)((i*13u+epoch*17u)%251u);
}
