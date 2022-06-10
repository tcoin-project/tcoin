#include "tcoin.h"
#include "stdlib.h"

entrypoint_t regularStart() {
  syscall::markJumpDest(reinterpret_cast<void *>(entrypoint));
  return entrypoint;
}

start_t _start(void *initData) {
  init(initData);
  return regularStart;
}

void serialize(char *&ptr, uint64_t x) {
  *reinterpret_cast<uint64_t *>(ptr) = x;
  ptr += sizeof(uint64_t);
}

void deserialize(char *&ptr, uint64_t &x) {
  x = *reinterpret_cast<uint64_t *>(ptr);
  ptr += sizeof(uint64_t);
}

void serialize(char *&ptr, const Address &x) {
  memcpyAligned(ptr, x, ADDR_LEN);
  ptr += ADDR_LEN;
}

void deserialize(char *&ptr, Address &x) {
  memcpyAligned(x.s, ptr, ADDR_LEN);
  ptr += ADDR_LEN;
}

uint64_t Address::balance() { return syscall::balance(this); }
bool Address::transfer(uint64_t value, const char *msg) {
  return syscall::transfer(this, value, msg, strlen(msg));
}

void require(bool cond, const char *revertMsg) {
  if (!cond)
    revert(revertMsg);
}

#ifndef NO_MALLOC
void *malloc(size_t n) {
  PRIVATE_DATA static char *ptr = 0;
  if (!ptr) {
    ptr = reinterpret_cast<char *>(
        ((reinterpret_cast<uint64_t>(malloc) >> 28) ^ 2) << 28);
  }
  n = (n + 7) & ~7ull;
  char *res = ptr;
  ptr += n;
  return res;
}

void *mallocShared(size_t n) {
  PRIVATE_DATA static char *ptr = 0;
  if (!ptr) {
    ptr = reinterpret_cast<char *>(
        ((reinterpret_cast<uint64_t>(mallocShared) >> 28) ^ 4) << 28);
  }
  n = (n + 7) & ~7ull;
  char *res = ptr;
  ptr += n;
  return res;
}
#endif