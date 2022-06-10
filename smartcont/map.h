#ifndef _MAP_H
#define _MAP_H

#include "stdlib.h"
#include "tcoin.h"

template <typename KeyType, typename ValueType> struct storageMap {
  Address mapId;
  storageMap(Address mapId) : mapId(mapId) {}
  storageMap(uint64_t id) : mapId(id) {}
  template <typename T> struct valueProxy {
    Address pos;
    operator T() {
      static_assert(serializeLen(T()) <= ADDR_LEN,
                    "map length can't be larger than maximum");
      char tmp[ADDR_LEN], *ptr = tmp;
      T res;
      storage::load(pos, tmp);
      deserialize(ptr, res);
      return res;
    }
    const ValueType &operator=(const T &x) {
      static_assert(serializeLen(T()) <= ADDR_LEN,
                    "map length can't be larger than maximum");
      char tmp[ADDR_LEN], *ptr = tmp;
      serialize(ptr, x);
      storage::store(pos, tmp);
      return x;
    }
  };
  template <typename K2, typename V2> struct valueProxy<storageMap<K2, V2>> {
    Address pos;
    auto operator[](const K2 &k) { return storageMap<K2, V2>(pos)[k]; }
  };
  valueProxy<ValueType> operator[](const KeyType &k) {
    const size_t len = serializeLen(mapId) + serializeLen(k);
    char tmp[len], *ptr = tmp;
    serialize(ptr, mapId);
    serialize(ptr, k);
    valueProxy<ValueType> res;
    crypto::sha256(tmp, len, res.pos.s);
    return res;
  }
};

#endif