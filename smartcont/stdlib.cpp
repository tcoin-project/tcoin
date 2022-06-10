#include "stdlib.h"

void memsetAligned(void *str, int c, size_t n) {
  char *ptr = reinterpret_cast<char *>(str);
  size_t t = n >> 3;
  n &= 7;
  uint64_t v = c;
  v += v << 8;
  v += v << 16;
  v += v << 32;
  for (; t; ptr += 8, t--)
    *reinterpret_cast<uint64_t *>(ptr) = v;
}

void memcpyAligned(void *str1, const void *str2, size_t n) {
  char *ptr1 = reinterpret_cast<char *>(str1);
  const char *ptr2 = reinterpret_cast<const char *>(str2);
  size_t t = n >> 3;
  n &= 7;
  for (; t; ptr1 += 8, ptr2 += 8, t--)
    *reinterpret_cast<uint64_t *>(ptr1) =
        *reinterpret_cast<const uint64_t *>(ptr2);
}

void *memset(void *str, int c, size_t n) {
  char *ptr = reinterpret_cast<char *>(str);
  for (; (reinterpret_cast<uint64_t>(str) & 7) && n; ptr++, n--)
    *ptr = c;
  if (n >= 8)
    memsetAligned(ptr, c, n);
  for (; n; ptr++, n--)
    *ptr = c;
  return str;
}

void *memcpy(void *str1, const void *str2, size_t n) {
  char *ptr1 = reinterpret_cast<char *>(str1);
  const char *ptr2 = reinterpret_cast<const char *>(str2);
  if ((reinterpret_cast<uint64_t>(str1) & 7) !=
      (reinterpret_cast<uint64_t>(str2) & 7)) {
    for (; n; ptr1++, ptr2++, n--)
      *ptr1 = *ptr2;
    return str1;
  }
  for (; (reinterpret_cast<uint64_t>(str1) & 7) && n; ptr1++, ptr2++, n--)
    *ptr1 = *ptr2;
  if (n >= 8)
    memcpyAligned(ptr1, ptr2, n);
  for (; n; ptr1++, ptr2++, n--)
    *ptr1 = *ptr2;
  return str1;
}

size_t strlen(const char *str) {
  size_t res = 0;
  while (*str)
    str++, res++;
  return res;
}