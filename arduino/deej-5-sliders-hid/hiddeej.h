#pragma once
#include "hidusb.h"
#include "deej-hid-definition.h"

class HIDdeej : public HIDusb
{
public:
    HIDdeej(uint8_t adc_bit_count = 10);
    bool begin(char* str = nullptr);

    void sendSlider(size_t sliderNumber, size_t sliderValue);
    void sendAll(size_t* sliderValues);

private:
    void sendReport();
    uint8_t truncate(size_t value);
    DeejVolumeArray report;
    uint8_t bit_count;
};
