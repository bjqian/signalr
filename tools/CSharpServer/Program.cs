using CSharpServer.hubs;
using MessagePack;


// Creates serializer.
var bytes = new byte[] { 0x91, 0x06 };
var result=MessagePackSerializer.Deserialize<object>(bytes);
// Unpack from stream.


var builder = WebApplication.CreateBuilder(args);

var connectionString = Environment.GetEnvironmentVariable("connectionString");
builder.Services.AddSignalR()
    .AddAzureSignalR(options =>
    {
        options.ConnectionString = connectionString;
    });
var app = builder.Build();

app.MapHub<Chat>("/chat");

app.Run();

