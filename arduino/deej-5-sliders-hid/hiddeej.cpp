#include "hiddeej.h"
#include "byteswap.h"
#define EPNUM_HID   0x03

// pulled from deej-hid-definition.h. Line 6 (Descriptor Size)
#define DEEJ_REPORT_DESCRIPTOR_SIZE 39


HIDdeej::HIDdeej(uint8_t adc_bit_count)
{
  bit_count = adc_bit_count;
  report_id = DEEJ_VOLUME_ARRAY_ID;
  enableHID = true;
}

bool HIDdeej::begin(char* str)
{
    memcpy(&desc_configuration[total], hidReportDescriptor, sizeof(hidReportDescriptor));
    total += sizeof(hidReportDescriptor);
    count++;

    memcpy(&hid_report_desc[EspTinyUSB::hid_report_desc_len], (uint8_t *)hidReportDescriptor, sizeof(hidReportDescriptor));
    EspTinyUSB::hid_report_desc_len += DEEJ_REPORT_DESCRIPTOR_SIZE;
    log_d("begin len: %d", EspTinyUSB::hid_report_desc_len);

    return EspTinyUSB::begin(str, 6);
}

void HIDdeej::sendReport()
{
    if(tud_hid_ready()){
        int ret = write((uint8_t*)&report, sizeof(hidReportDescriptor));
        if(-1 == ret) log_e("error: %i", ret);
    }
}

uint8_t HIDdeej::truncate(size_t value)
{
    return bit_count > 8 ? ((uint8_t)(value >> (bit_count - 8))) : ((uint8_t)value);
}

void HIDdeej::sendSlider(size_t sliderNumber, size_t sliderValue)
{
    uint8_t truncatedValue = truncate(sliderValue);
    report.Payload[sliderNumber] = truncatedValue;
    sendReport();
}

void HIDdeej::sendAll(size_t* sliderValues)
{
    for (uint8_t i = 0; i < sizeof(DeejVolumeArray::Payload); i++)
    {
        report.Payload[i] = truncate(sliderValues[i]);
    }
    sendReport();
}
