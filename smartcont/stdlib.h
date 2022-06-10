#ifndef _STDLIB_H
#define _STDLIB_H

extern "C" {
typedef unsigned long long uint64_t;
typedef long long int64_t;
typedef unsigned int uint32_t;
typedef int int32_t;
typedef unsigned short uint16_t;
typedef short int16_t;
typedef unsigned char uint8_t;
typedef char int8_t;
typedef long long size_t;

void *memset(void *str, int c, size_t n);
void *memcpy(void *str1, const void *str2, size_t n);
void memsetAligned(void *str, int c, size_t n);
void memcpyAligned(void *str1, const void *str2, size_t n);

size_t strlen(const char *str);
}

#endif