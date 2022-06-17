#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "syscall.h"
#include "tcoin.h"

const size_t proxyCodeLen = 520;
const uint8_t proxyCode[proxyCodeLen] = {
    127, 69,  76,  70,  2,   1,   1,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   2,   0,   243, 0,   1,   0,   0,   0,   144, 1,   0,   16,  0,   0,
    0,   0,   64,  0,   0,   0,   0,   0,   0,   0,   136, 2,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   64,  0,   56,  0,   1,   0,   64,  0,
    6,   0,   5,   0,   1,   0,   0,   0,   5,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   16,  0,   0,   0,   0,   0,   0,
    0,   16,  0,   0,   0,   0,   8,   2,   0,   0,   0,   0,   0,   0,   8,
    2,   0,   0,   0,   0,   0,   0,   0,   16,  0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
    0,   0,   0,   0,   0,   0,   0,   0,   0,   0,   19,  1,   1,   255, 147,
    7,   16,  245, 35,  52,  17,  0,   147, 215, 23,  0,   183, 21,  0,   0,
    23,  5,   0,   0,   19,  5,   69,  2,   231, 128, 7,   0,   131, 48,  129,
    0,   147, 7,   5,   0,   23,  5,   0,   0,   19,  5,   5,   3,   19,  1,
    1,   1,   103, 128, 7,   0,   84,  113, 151, 209, 34,  189, 202, 207, 195,
    247, 160, 136, 20,  226, 42,  112, 188, 31,  212, 5,   248, 218, 53,  119,
    97,  216, 199, 190, 41,  209, 76,  202, 150, 2,   8,   166, 157, 4,   45,
    0,   220, 1,   112, 16,  193, 167, 156, 159, 52,  123, 176, 120, 110, 170,
    215, 34,  216, 112, 87,  97,  82,  161, 32,  125};

const Address *createExchange(const Address *token) {
  uint8_t tmp[proxyCodeLen];
  memcpyAligned(tmp, proxyCode, proxyCodeLen);
  memcpyAligned(tmp + 0x1E8, *token, ADDR_LEN);
  uint64_t tmp2[4];
  crypto::sha256(*token, ADDR_LEN, reinterpret_cast<char *>(tmp2));
  Address res = syscall::create(reinterpret_cast<char *>(tmp), proxyCodeLen,
                                CREATE_USENONCE, tmp2[0]);
  return asSharedPtr(res);
}

const Address *getExchange(const Address *token) {
  uint8_t tmp[ADDR_LEN + 8 * 2 + proxyCodeLen];
  char *t2 = reinterpret_cast<char *>(tmp) + ADDR_LEN;
  memcpyAligned(tmp, self(), ADDR_LEN);
  serialize(t2, uint64_t(CREATE_USENONCE));
  memcpyAligned(tmp + ADDR_LEN + 8 * 2, proxyCode, proxyCodeLen);
  memcpyAligned(tmp + ADDR_LEN + 8 * 2 + 0x1E8, *token, ADDR_LEN);
  uint64_t tmp2[4];
  crypto::sha256(*token, ADDR_LEN, reinterpret_cast<char *>(tmp2));
  serialize(t2, tmp2[0]);
  Address res;
  crypto::sha256(reinterpret_cast<char *>(tmp), ADDR_LEN + 8 * 2 + proxyCodeLen,
                 res.s);
  return asSharedPtr(res);
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(createExchange);
  export(getExchange);
  return 0;
}

void regularInit(const void *data) {}

void init() {}