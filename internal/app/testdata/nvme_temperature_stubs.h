
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/IOCFPlugIn.h>
typedef struct { char key[5]; char name[64]; float value; } temp_sensor_t;
typedef CFTypeRef IOHIDServiceClientRef;
typedef CFTypeRef IOHIDEventRef;
enum { kIOHIDEventTypeTemperature=15 };
static const double kSiliconMinTempC=10;
static temp_sensor_t g_nvme_temps[16];
static int g_nvme_temp_count, g_nvme_smart_active;
static int hid_available=1, enumeration_error=0;
static CFArrayRef services;
static CFArrayRef getCachedHIDServices(void) {
 return hid_available ? services : NULL;
}
static CFStringRef IOHIDServiceClientCopyProperty(IOHIDServiceClientRef s,CFStringRef key){return CFRetain(CFSTR("Apple NAND temp"));}
static IOHIDEventRef IOHIDServiceClientCopyEvent(IOHIDServiceClientRef s,int type,int x,int y){double t=42;return CFNumberCreate(NULL,kCFNumberDoubleType,&t);}
static double IOHIDEventGetFloatValue(IOHIDEventRef e,int field){double t;CFNumberGetValue(e,kCFNumberDoubleType,&t);return t;}
static kern_return_t mockMatching(mach_port_t port,CFDictionaryRef dict,io_iterator_t *iter){CFRelease(dict);*iter=0;return enumeration_error?kIOReturnError:kIOReturnSuccess;}
static io_object_t mockNext(io_iterator_t iter){return 0;}
static kern_return_t mockRelease(io_object_t obj){return 0;}
#define IOServiceGetMatchingServices mockMatching
#define IOIteratorNext mockNext
#define IOObjectRelease mockRelease
