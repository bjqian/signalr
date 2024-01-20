using CSharpCommon;
using Microsoft.AspNetCore.SignalR;

namespace CSharpServer.hubs;

public class Chat: Hub
{
    public async Task<bool> Hi(string message)
    {
        Console.WriteLine(message);
        return true;
    } 
}