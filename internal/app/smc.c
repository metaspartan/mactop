// smc.c
#include "smc.h"
#include <stdio.h>
#include <string.h>

static unsigned int SMCFourCC(const char *value) {
  return ((unsigned int)(unsigned char)value[0] << 24) |
         ((unsigned int)(unsigned char)value[1] << 16) |
         ((unsigned int)(unsigned char)value[2] << 8) |
         (unsigned int)(unsigned char)value[3];
}

static int SMCIsType(unsigned int dataType, const char *type) {
  return dataType == SMCFourCC(type);
}

io_connect_t SMCOpen(void) {
  kern_return_t result;
  io_iterator_t iterator;
  io_object_t device;
  io_connect_t conn = 0;

  CFMutableDictionaryRef matchingDictionary = IOServiceMatching("AppleSMC");
  result = IOServiceGetMatchingServices(kIOMainPortDefault, matchingDictionary,
                                        &iterator);
  if (result != kIOReturnSuccess) {
    return 0;
  }

  device = IOIteratorNext(iterator);
  IOObjectRelease(iterator);

  if (device == 0) {
    return 0;
  }

  result = IOServiceOpen(device, mach_task_self(), 0, &conn);
  IOObjectRelease(device);

  if (result != kIOReturnSuccess) {
    return 0;
  }

  return conn;
}

kern_return_t SMCClose(io_connect_t conn) { return IOServiceClose(conn); }

kern_return_t SMCCall(io_connect_t conn, int index,
                      SMCKeyData_t *inputStructure,
                      SMCKeyData_t *outputStructure) {
  size_t structureInputSize;
  size_t structureOutputSize;

  structureInputSize = sizeof(SMCKeyData_t);
  structureOutputSize = sizeof(SMCKeyData_t);

  return IOConnectCallStructMethod(conn, index, inputStructure,
                                   structureInputSize, outputStructure,
                                   &structureOutputSize);
}

static kern_return_t SMCCallChecked(io_connect_t conn, int index,
                                    SMCKeyData_t *inputStructure,
                                    SMCKeyData_t *outputStructure) {
  kern_return_t result =
      SMCCall(conn, index, inputStructure, outputStructure);
  if (result != kIOReturnSuccess) {
    return result;
  }
  if (outputStructure->result != 0) {
    return kIOReturnError;
  }
  return kIOReturnSuccess;
}

kern_return_t SMCReadKey(io_connect_t conn, const char *key,
                         SMCKeyData_t *val) {
  kern_return_t result;
  SMCKeyData_t inputStructure;
  SMCKeyData_t outputStructure;

  memset(&inputStructure, 0, sizeof(SMCKeyData_t));
  memset(&outputStructure, 0, sizeof(SMCKeyData_t));
  memset(val, 0, sizeof(SMCKeyData_t));

  inputStructure.key = (key[0] << 24) | (key[1] << 16) | (key[2] << 8) | key[3];
  inputStructure.data8 = SMC_CMD_READ_KEYINFO;

  result =
      SMCCallChecked(conn, KERNEL_INDEX_SMC, &inputStructure, &outputStructure);
  if (result != kIOReturnSuccess) {
    return result;
  }

  val->keyInfo.dataSize = outputStructure.keyInfo.dataSize;
  val->keyInfo.dataType = outputStructure.keyInfo.dataType;
  inputStructure.keyInfo.dataSize = val->keyInfo.dataSize;
  inputStructure.data8 = SMC_CMD_READ_BYTES;

  result =
      SMCCallChecked(conn, KERNEL_INDEX_SMC, &inputStructure, &outputStructure);
  if (result != kIOReturnSuccess) {
    return result;
  }

  memcpy(val->bytes, outputStructure.bytes, sizeof(outputStructure.bytes));
  return kIOReturnSuccess;
}

double SMCGetFloatValue(io_connect_t conn, const char *key) {
  SMCKeyData_t val;
  kern_return_t result = SMCReadKey(conn, key, &val);
  if (result != kIOReturnSuccess) {
    return 0.0;
  }

  if (SMCIsType(val.keyInfo.dataType, "flt ") &&
      val.keyInfo.dataSize >= sizeof(float)) {
    float f;
    memcpy(&f, val.bytes, 4);
    return (double)f;
  }
  if (SMCIsType(val.keyInfo.dataType, "ui8 ") &&
      val.keyInfo.dataSize >= 1) {
    return (double)(unsigned char)val.bytes[0];
  }

  return 0.0;
}

int SMCGetKeyCount(io_connect_t conn) {
  SMCKeyData_t val;
  kern_return_t result = SMCReadKey(conn, "#KEY", &val);
  if (result != kIOReturnSuccess) {
    // printf("SMCGetKeyCount: SMCReadKey failed with result %d\n", result);
    return 0;
  }

  unsigned int count = 0;
  count = ((unsigned char)val.bytes[0] << 24) |
          ((unsigned char)val.bytes[1] << 16) |
          ((unsigned char)val.bytes[2] << 8) | (unsigned char)val.bytes[3];
  // printf("SMCGetKeyCount: Found %d keys\n", count);
  return count;
}

kern_return_t SMCGetKeyFromIndex(io_connect_t conn, int index,
                                 char *outputKey) {
  kern_return_t result;
  SMCKeyData_t inputStructure;
  SMCKeyData_t outputStructure;

  memset(&inputStructure, 0, sizeof(SMCKeyData_t));
  memset(&outputStructure, 0, sizeof(SMCKeyData_t));

  inputStructure.data8 = SMC_CMD_READ_INDEX;
  inputStructure.data32 = index;

  result =
      SMCCallChecked(conn, KERNEL_INDEX_SMC, &inputStructure, &outputStructure);
  if (result != kIOReturnSuccess) {
    return result;
  }

  unsigned int key = outputStructure.key;
  outputKey[0] = (key >> 24) & 0xFF;
  outputKey[1] = (key >> 16) & 0xFF;
  outputKey[2] = (key >> 8) & 0xFF;
  outputKey[3] = key & 0xFF;
  outputKey[4] = '\0';

  return kIOReturnSuccess;
}

kern_return_t SMCGetKeyInfo(io_connect_t conn, const char *key,
                            SMCKeyData_keyInfo_t *keyInfo) {
  kern_return_t result;
  SMCKeyData_t inputStructure;
  SMCKeyData_t outputStructure;

  memset(&inputStructure, 0, sizeof(SMCKeyData_t));
  memset(&outputStructure, 0, sizeof(SMCKeyData_t));

  inputStructure.key = (key[0] << 24) | (key[1] << 16) | (key[2] << 8) | key[3];
  inputStructure.data8 = SMC_CMD_READ_KEYINFO;

  result =
      SMCCallChecked(conn, KERNEL_INDEX_SMC, &inputStructure, &outputStructure);
  if (result != kIOReturnSuccess) {
    return result;
  }

  *keyInfo = outputStructure.keyInfo;
  return kIOReturnSuccess;
}

kern_return_t SMCWriteKey(io_connect_t conn, const char *key,
                          unsigned int dataType, SMCBytes_t bytes,
                          unsigned int dataSize) {
  SMCKeyData_t inputStructure;
  SMCKeyData_t outputStructure;

  memset(&inputStructure, 0, sizeof(SMCKeyData_t));
  memset(&outputStructure, 0, sizeof(SMCKeyData_t));

  inputStructure.key = (key[0] << 24) | (key[1] << 16) | (key[2] << 8) | key[3];
  inputStructure.data8 = SMC_CMD_WRITE_BYTES;
  inputStructure.keyInfo.dataSize = dataSize;
  inputStructure.keyInfo.dataType = dataType;
  memcpy(inputStructure.bytes, bytes, dataSize);

  return SMCCallChecked(conn, KERNEL_INDEX_SMC, &inputStructure,
                        &outputStructure);
}

kern_return_t SMCSetFloat(io_connect_t conn, const char *key, float value) {
  // First read the key info to get the correct data type and size
  SMCKeyData_keyInfo_t keyInfo;
  kern_return_t result = SMCGetKeyInfo(conn, key, &keyInfo);
  if (result != kIOReturnSuccess) {
    return result;
  }

  SMCBytes_t bytes;
  memset(bytes, 0, sizeof(bytes));
  if (SMCIsType(keyInfo.dataType, "flt ") &&
      keyInfo.dataSize >= sizeof(float)) {
    memcpy(bytes, &value, sizeof(float));
  } else if (SMCIsType(keyInfo.dataType, "ui8 ") &&
             keyInfo.dataSize >= 1) {
    if (value < 0.0f || value > 255.0f) {
      return kIOReturnBadArgument;
    }
    bytes[0] = (char)(unsigned char)value;
  } else {
    return kIOReturnUnsupported;
  }

  return SMCWriteKey(conn, key, keyInfo.dataType, bytes, keyInfo.dataSize);
}
