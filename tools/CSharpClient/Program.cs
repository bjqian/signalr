// See https://aka.ms/new-console-template for more information

using Microsoft.AspNetCore.SignalR.Client;
using Microsoft.Extensions.Logging;

Console.WriteLine("Hello, World!");

// local for go
var url = "http://localhost:8080/chat";

var connection = new HubConnectionBuilder()
    .WithUrl(url, o =>
    {
        o.SkipNegotiation = false;

        o.Transports = Microsoft.AspNetCore.Http.Connections.HttpTransportType.ServerSentEvents;
        o.UseStatefulReconnect = true;
    }).ConfigureLogging(l =>
    {
        l.SetMinimumLevel(LogLevel.Debug);
        l.AddConsole();
    })
    .Build();

connection.KeepAliveInterval = TimeSpan.FromSeconds(1000);
connection.HandshakeTimeout = TimeSpan.FromSeconds(10000);
connection.ServerTimeout = TimeSpan.FromSeconds(10000);
connection.Closed += async (error) =>
{
    Console.WriteLine("Connection closed");
};

try
{
    await connection.StartAsync();
}
catch (Exception ex)
{
    Console.WriteLine(ex.Message);
}

connection.On<string>("Receive", (message) =>
{
    Console.WriteLine(message);
});

try
{
    // local for go
    await connection.SendAsync("Hi", "send from .NET 8");
    var result= await connection.InvokeAsync<bool>("Hi", "invoke from .NET 8");
    Console.WriteLine($"Invoke result: {result}");   
}catch(Exception ex)
{
    Console.WriteLine(ex.Message);
}

Console.WriteLine("Connection started");

var state= connection.State;
Console.WriteLine(connection.State);

while (true)
{
    await Task.Delay(1000);
    if(connection.State!=state)
    {
        Console.WriteLine(connection.State);
        state=connection.State;
    }
}