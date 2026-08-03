//go:build darwin

#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

NSString * const TerminalNotifierBundleID = @"com.apple.terminal";
NSString *_fakeBundleIdentifier = TerminalNotifierBundleID;

@implementation NSBundle (FakeBundleIdentifier)

- (NSString *)__bundleIdentifier
{
    if (self == [NSBundle mainBundle]) {
        return _fakeBundleIdentifier;
    }

    return [self __bundleIdentifier];
}

@end

static BOOL installFakeBundleIdentifierHook()
{
    Class c = objc_getClass("NSBundle");
    if (c) {
        method_exchangeImplementations(class_getInstanceMethod(c, @selector(bundleIdentifier)),
                                       class_getInstanceMethod(c, @selector(__bundleIdentifier)));
        return YES;
    }

    return NO;
}

@interface NotificationCenterDelegate : NSObject <NSUserNotificationCenterDelegate>

@property (nonatomic, assign) BOOL didDeliver;

@end

@implementation NotificationCenterDelegate

- (void)userNotificationCenter:(NSUserNotificationCenter *)center didDeliverNotification:(NSUserNotification *)notification
{
    self.didDeliver = YES;
}

- (BOOL)userNotificationCenter:(NSUserNotificationCenter *)center shouldPresentNotification:(NSUserNotification *)notification
{
    return YES;
}

@end


void Push(const char *strJson)
{
    NSDictionary *mData = [NSJSONSerialization
                          JSONObjectWithData:[[NSString stringWithUTF8String:strJson]
                                              dataUsingEncoding:NSUTF8StringEncoding]
                          options:0
                          error:nil];

    if (installFakeBundleIdentifierHook()) {
        if (![@"" isEqualToString:mData[@"bundle_id"]]){
            _fakeBundleIdentifier = mData[@"bundle_id"];
        }
    }

    NSUserNotificationCenter *nc = [NSUserNotificationCenter defaultUserNotificationCenter];
    NotificationCenterDelegate *ncDelegate = [[NotificationCenterDelegate alloc] init];
    ncDelegate.didDeliver = NO;
    nc.delegate = ncDelegate;

    NSUserNotification *notice = [[NSUserNotification alloc] init];
    notice.title = mData[@"title"];
    notice.subtitle = mData[@"subtitle"];
    notice.informativeText = mData[@"message"];
    if (![@"" isEqualToString:mData[@"audio"]]){
        notice.soundName = mData[@"audio"];
    }

    [nc deliverNotification:notice];

    int i = 0;
    while (!ncDelegate.didDeliver) {
        [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];
        i++;
        if (i > 2000) {
            break;
        }
    }
}
