#include <WiFi.h>
#include <AsyncTCP.h>
#include <ESPAsyncWebSrv.h>
#include <mdns.h>

// CONSTANTS
const int NUM_SLIDERS = 5;

const char WIFI_SSID[] = "EXAMPLE";
const char WIFI_PASSWORD[] = "EXAMPLE";

const char WS_ENDPOINT[] = "/ws";
const char WS_HOSTNAME[] = "deej";
const uint WS_PORT = 80;
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
AsyncWebServer webServer(WS_PORT);
AsyncWebSocket webSocket(WS_ENDPOINT);

// FNS

void handleWebSocketMessage(void *arg, uint8_t *data, size_t len) {
  AwsFrameInfo *info = (AwsFrameInfo*)arg;
  if (info->final && info->index == 0 && info->len == len && info->opcode == WS_TEXT) {
    data[len] = 0;
    Serial.printf("Got message: %s\r\n", data);
  }
}

void onEvent(AsyncWebSocket *server, AsyncWebSocketClient *client, AwsEventType type,
             void *arg, uint8_t *data, size_t len) {
  switch (type) {
    case WS_EVT_CONNECT:
      Serial.printf("WebSocket client #%u connected from %s\n", client->id(), client->remoteIP().toString().c_str());
      break;
    case WS_EVT_DISCONNECT:
      Serial.printf("WebSocket client #%u disconnected\n", client->id());
      break;
    case WS_EVT_DATA:
      handleWebSocketMessage(arg, data, len);
      break;
    case WS_EVT_PONG:
    case WS_EVT_ERROR:
      break;
  }
}

void setup()
{
  for (int i = 0; i < NUM_SLIDERS; i++)
  {
    pinMode(analogInputs[i], INPUT);
  }

  Serial.begin(9600);

  WiFi.mode(WIFI_STA);

  Serial.println("Connecting to WiFi");
  esp_err_t err = mdns_init();
  if (err != ESP_OK) {
    Serial.printf("Failed to start mdns! Error: %s\r\n", esp_err_to_name(err));
    esp_restart();
  }
  mdns_hostname_set(WS_HOSTNAME);
  mdns_instance_name_set("Deej application audio controller");
  WiFi.setHostname(WS_HOSTNAME);
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
  webSocket.onEvent(onEvent);
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

  Serial.println("Cleaning up clients!");
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
