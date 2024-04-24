#define CFG_TUD_HID 1
#include "deej-hid-definition.h"
#include "hiddeej.h"
// CONSTANTS
const int NUM_SLIDERS = 5;

const int analogInputs[NUM_SLIDERS] = {9, 10, 11, 12, 13};
uint16_t analogSliderValues[NUM_SLIDERS];
uint16_t oldAnalogSliderValues[NUM_SLIDERS];
bool sliderChange[NUM_SLIDERS];
unsigned long lastPublishTime = 0;
unsigned long lastClientCleanup = 0;

// GLOBALS
char HIDDescription[] = "Deej volume control";

HIDdeej hidDeej(10);

// FNS


void setup()
{
  Serial.begin(9600);
  hidDeej.begin(HIDDescription);
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
    printSliderValues(); // For debug
    sendSliderValues();
  }
  delay(10);
}

void updateSliderValues()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    analogSliderValues[i] = (int)(analogRead(analogInputs[i]));
    // analogSliderValues[i] = (int)(((u_int64_t)analogRead(analogInputs[i])) * 1023 / 4095);

    // since we will be using wifi, we want to cut out jitter at the source.
    // If your sliders have a travel of 60mm and range from 0 to 1023 in value
    // this would be equivalent to ignoring any slider changes less than or equal to 0.18mm (7 thousandths of an inch)
    const int totalChange = (int)oldAnalogSliderValues[i] - (int)analogSliderValues[i];

    const bool significantSliderChange = totalChange <= -40 || totalChange >= 40;

    if (significantSliderChange)
    {
      oldAnalogSliderValues[i] = analogSliderValues[i];
      sliderChange[i] = true;
    }
  }
}

void sendSliderValues()
{
    size_t allSliderValues[NUM_SLIDERS] = {};
    for (int i = 0; i < NUM_SLIDERS; i++) {
        allSliderValues[i] = (size_t)analogSliderValues[i];
        sliderChange[i] = false;
    }

    hidDeej.sendAll(allSliderValues);

    return;
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
