// #define CONFIG_LOG_MASTER_LEVEL
#ifdef CORE_DEBUG_LEVEL
#undef CORE_DEBUG_LEVEL
#endif

#define CORE_DEBUG_LEVEL 3
#define CONFIG_LOG_DEFAULT_LEVEL ESP_LOG_DEBUG

#include <stdint.h>
#include "deej-hid-definition.h"
#include "bluedeej.h"

// CONSTANTS
const int NUM_SLIDERS = 5;
#define ADC_BIT_RESOLUTION 12
typedef uint16_t slider_t;

const int analogInputs[NUM_SLIDERS] = {9, 10, 11, 12, 13};
slider_t analogSliderValues[NUM_SLIDERS];
uint8_t truncatedSliderValues[NUM_SLIDERS];
bool sliderChange[NUM_SLIDERS];
unsigned long lastPublishTime = 0;
unsigned long lastClientCleanup = 0;

// GLOBALS

BleDeej deej;

// FNS

uint8_t truncateSliderValue(slider_t rawValue)
{
    if (ADC_BIT_RESOLUTION <= 8) return (uint8_t)rawValue;

    return (uint8_t)(rawValue >> (ADC_BIT_RESOLUTION - 8));
}

void setup()
{
    Serial.begin(9600);
    deej.begin();
}

bool shouldUpdateSliders()
{
  bool atLeastOneSliderChange = false;
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    if (sliderChange[i])
    {
      atLeastOneSliderChange = true;
      break;
    }
  }
  return atLeastOneSliderChange;
}

void loop()
{
  updateSliderValues();
  if (shouldUpdateSliders()) {
    bool sliderValuesSent = sendSliderValues();
    if (sliderValuesSent) printSliderValues(); // For debug
  }
  delay(10);
}

void updateSliderValues()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    analogSliderValues[i] = analogRead(analogInputs[i]);
    // analogSliderValues[i] = (int)(((u_int64_t)analogRead(analogInputs[i])) * 1023 / 4095);

    // since we will be using wifi, we want to cut out jitter at the source.
    // If your sliders have a travel of 60mm and range from 0 to 1023 in value
    // this would be equivalent to ignoring any slider changes less than or equal to 0.18mm (7 thousandths of an inch)
    const uint8_t truncatedValue = truncateSliderValue(analogSliderValues[i]);

    if (truncatedValue != truncatedSliderValues[i])
    {
      truncatedSliderValues[i] = truncatedValue;
      sliderChange[i] = true;
    }
  }
}

bool sendSliderValues()
{
    if (deej.isConnected() == false) { return false; }

    DeejVolumeArray report;
    for (size_t i = 0; i < NUM_SLIDERS; i++) {
        report.Payload[i] = truncatedSliderValues[i];
        sliderChange[i] = false;
    }
    deej.sendReport(&report);
    return true;
}

void printSliderValues()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    String printedString = String("Slider #") + String(i + 1) + String(": ") + String(analogSliderValues[i]) + String(" mV, ") + String("Changed: ") + String(sliderChange[i]);
    Serial.write(printedString.c_str());

    if (i < NUM_SLIDERS - 1)
    {
      Serial.write(" | ");
    }
    else
    {
      Serial.write("\n");
    }
  }
}
