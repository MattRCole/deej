#include <WiFi.h>
#include <AsyncTCP.h>
#include <ESPAsyncWebSrv.h>

// CONSTANTS
const int NUM_SLIDERS = 5;

const char WIFI_HOSTNAME[] = "deej";
const char WIFI_SSID[] = "EXAMPLE";
const char WIFI_PASSWORD[] = "EXAMPLE";

const char WS_ENDPOINT[] = "/ws";
const unsigned int WS_MIN_PUBLISH_TIME = 50; // 50ms
const unsigned int WS_CLEANUP_INTERVAL = 1000; // 1s

// GLOBALS

const int analogInputs[NUM_SLIDERS] = {9, 10, 11, 12, 13};
int analogSliderValues[NUM_SLIDERS];
int oldAnalogSliderValues[NUM_SLIDERS];
bool sliderChange[NUM_SLIDERS];
unsigned long lastPublishTime = 0;
unsigned long lastClientCleanup = 0;

WiFiClient wifiClient;
AsyncWebServer webServer(80);
AsyncWebSocket webSocket(WS_ENDPOINT);

// FNS

void setup()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    pinMode(analogInputs[i], INPUT);
  }

  Serial.begin(9600);

  WiFi.mode(WIFI_STA);

  Serial.println("Connecting to WiFi");
  // esp_err_t err = mdns
  WiFi.setHostname(WIFI_HOSTNAME);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  unsigned int count = 0;
  while (WiFi.status() != WL_CONNECTED)
  {
    delay(500);
    Serial.print(".");
    Serial.flush();
    count++;
    if (count > 20)
    {
      Serial.println("");
      Serial.println("Cannot connect to wifi, restarting!");
      esp_restart();
    }
  }
  Serial.println("");
  Serial.println("Connected!");

  Serial.println("Starting Websocket server");
  webServer.addHandler(&webSocket);
  webServer.begin();
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
  return atLeastOneSliderChange && millis() - lastPublishTime >= WS_MIN_PUBLISH_TIME;
}

void cleanupClients() {
  if (millis() - lastClientCleanup <= WS_CLEANUP_INTERVAL)
    return;

  webSocket.cleanupClients();
  lastClientCleanup = millis();
}

void loop()
{
  cleanupClients();
  updateSliderValues();
  if (shouldUpdateSliders()) {
    printSliderValues(); // For debug
    Serial.printf("Sending update to %d clients\r\n", webSocket.count());
    sendSliderValues();
  }
  delay(10);
}

void updateSliderValues()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    analogSliderValues[i] = (int)(((u_int64_t)analogRead(analogInputs[i])) * 1023 / 4095);

    // since we will be using wifi, we want to cut out jitter at the source.
    // If your sliders have a travel of 60mm and range from 0 to 1023 in value
    // this would be equivalent to ignoring any slider changes less than or equal to 0.18mm (7 thousandths of an inch)
    const bool significantSliderChange = oldAnalogSliderValues[i] <= analogSliderValues[i] - 3 || oldAnalogSliderValues[i] >= analogSliderValues[i] + 3;

    if (significantSliderChange)
    {
      oldAnalogSliderValues[i] = analogSliderValues[i];
      sliderChange[i] = true;
    }
  }
}

void sendSliderValues()
{
  String builtString = String("");

  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    // since we're sending the values, clear out all changes
    sliderChange[i] = false;

    builtString += String((int)analogSliderValues[i]);

    if (i < NUM_SLIDERS - 1)
    {
      builtString += String("|");
    }
  }
  lastPublishTime = millis();
  webSocket.textAll(builtString);
}

void printSliderValues()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    String printedString = String("Slider #") + String(i + 1) + String(": ") + String(analogSliderValues[i]) + String(" mV");
    Serial.write(printedString.c_str());

    if (i < NUM_SLIDERS - 1)
    {
      Serial.write(" | ");
    }
    // else
    // {
    //   Serial.write("\n");
    // }
  }
}
